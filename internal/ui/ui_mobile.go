// Copyright 2016 Hajime Hoshi
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build android ios

package ui

import (
	"errors"
	"image"
	"runtime"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/internal/devicescale"
	"github.com/hajimehoshi/ebiten/internal/opengl"
)

func Render(chError <-chan error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if chError == nil {
		return errors.New("ui: chError must not be nil")
	}
	// TODO: Check this is called on the rendering thread
	select {
	case <-chPauseStart:
		if err := doGLWorks(chError, chPauseEnd); err != nil {
			return err
		}
		chPauseEnd2 <- struct{}{}
		return nil
	case <-chResumeStart:
		if err := doGLWorks(chError, chResumeEnd); err != nil {
			return err
		}
		return nil
	case chRender <- struct{}{}:
		return opengl.GetContext().DoWork(chError, chRenderEnd)
	case <-time.After(500 * time.Millisecond):
		// This function must not be blocked so we need to break after a while.
		return nil
	}
}

type userInterface struct {
	width       int
	height      int
	scale       float64
	sizeChanged bool
	paused      bool
}

var (
	// TODO: Rename these channels
	chRender      = make(chan struct{})
	chRenderEnd   = make(chan struct{})
	chPause       = make(chan struct{})
	chPauseStart  = make(chan struct{})
	chPauseEnd    = make(chan struct{})
	chPauseEnd2   = make(chan struct{})
	chResume      = make(chan struct{})
	chResumeStart = make(chan struct{})
	chResumeEnd   = make(chan struct{})
	currentUI     = &userInterface{
		sizeChanged: true,
		paused:      false,
	}
)

func Run(width, height int, scale float64, title string, g GraphicsContext) error {
	u := currentUI

	u.m.Lock()
	u.width = width
	u.height = height
	u.scale = scale
	u.sizeChanged = true
	u.m.Unlock()
	// title is ignored?

	initOpenGL()

	for {
		if err := u.update(g); err != nil {
			return err
		}
	}
}

func (u *userInterface) updateGraphicsContext(g GraphicsContext) {
	sizeChanged := false
	width, height := 0, 0
	actualScale := 0.0

	u.m.Lock()
	sizeChanged = u.sizeChanged
	if sizeChanged {
		width = u.width
		height = u.height
		actualScale = u.actualScaleImpl()
	}
	u.sizeChanged = false
	u.m.Unlock()

	if sizeChanged {
		// Sizing also calls GL functions
		g.SetSize(width, height, actualScale)
	}
}

func actualScale() float64 {
	return currentUI.actualScale()
}

func (u *userInterface) actualScale() float64 {
	u.m.Lock()
	s := u.actualScaleImpl()
	u.m.Unlock()
	return s
}

func (u *userInterface) actualScaleImpl() float64 {
	scale := u.scale
	if u.fullscreenScale != 0 {
		scale = u.fullscreenScale
	}
	return scale * devicescale.DeviceScale()
}

func (u *userInterface) update(g GraphicsContext) error {
	<-chRender
	defer func() {
		chRenderEnd <- struct{}{}
	}()

	u.updateGraphicsContext(g)

	if err := g.Update(func() {
		u.updateGraphicsContext(g)
	}); err != nil {
		return err
	}
	return nil
}

func screenSize() (int, int) {
	return currentUI.screenSize()
}

func (u *userInterface) screenSize() (int, int) {
	u.m.Lock()
	w, h := u.width, u.height
	u.m.Unlock()
	return w, h
}

func SetScreenSize(width, height int) bool {
	currentUI.setScreenSize(width, height)
	return true
}

func (u *userInterface) setScreenSize(width, height int) {
	u.m.Lock()
	if u.width != width || u.height != height {
		u.width = width
		u.height = height
		u.updateFullscreenScaleIfNeeded()
		u.sizeChanged = true
	}
	u.m.Unlock()
}

func SetScreenScale(scale float64) bool {
	currentUI.setScreenScale(scale)
	return false
}

func (u *userInterface) setScreenScale(scale float64) {
	u.m.Lock()
	if u.scale != scale {
		u.scale = scale
		u.sizeChanged = true
	}
	u.m.Unlock()
}

func ScreenScale() float64 {
	u := currentUI
	u.m.RLock()
	s := u.scale
	u.m.RUnlock()
	return s
}

func setFullscreen(widthPx, heightPx int) {
	currentUI.setFullscreen(widthPx, heightPx)
}

func (u *userInterface) setFullscreen(widthPx, heightPx int) {
	u.m.Lock()
	u.fullscreenWidthPx = widthPx
	u.fullscreenHeightPx = heightPx
	u.updateFullscreenScaleIfNeeded()
	u.sizeChanged = true
	u.m.Unlock()
}

func (u *userInterface) Update() (interface{}, error) {
	// TODO: Need lock?
	if u.sizeChanged {
		u.sizeChanged = false
		e := ScreenSizeEvent{
			Width:       u.width,
			Height:      u.height,
			Scale:       u.scale,
			ActualScale: u.actualScreenScale(),
		}
		return e, nil
	}
	if u.paused {
		select {
		case <-chResume:
			u.paused = false
			chResumeStart <- struct{}{}
			return ResumeEvent{chResumeEnd}, nil
		}
	}
	u.fullscreenScale = scale / devicescale.DeviceScale()
}

func ScreenPadding() (x0, y0, x1, y1 float64) {
	return currentUI.screenPadding()
}

func (u *userInterface) screenPadding() (x0, y0, x1, y1 float64) {
	u.m.Lock()
	x0, y0, x1, y1 = u.screenPaddingImpl()
	u.m.Unlock()
	return
}

func (u *userInterface) screenPaddingImpl() (x0, y0, x1, y1 float64) {
	if u.fullscreenScale == 0 {
		return 0, 0, 0, 0
	}
	s := u.fullscreenScale * devicescale.DeviceScale()
	ox := (float64(u.fullscreenWidthPx) - float64(u.width)*s) / 2
	oy := (float64(u.fullscreenHeightPx) - float64(u.height)*s) / 2
	return ox, oy, ox, oy
}

func adjustCursorPosition(x, y int) (int, int) {
	return currentUI.adjustCursorPosition(x, y)
}

func (u *userInterface) adjustCursorPosition(x, y int) (int, int) {
	u.m.Lock()
	ox, oy, _, _ := u.screenPaddingImpl()
	s := u.actualScaleImpl()
	u.m.Unlock()
	return x - int(ox/s), y - int(oy/s)
}

func IsCursorVisible() bool {
	return false
}

func SetCursorVisible(visible bool) {
	// Do nothing
}

func IsFullscreen() bool {
	return false
}

func SetFullscreen(fullscreen bool) {
	// Do nothing
}

func IsRunnableInBackground() bool {
	return false
}

func SetRunnableInBackground(runnableInBackground bool) {
	// Do nothing
}

func Pause() error {
	// Pause must be done in the current GL context.
	chPause <- struct{}{}
	<-chPauseEnd2
	return nil
}

func Resume() error {
	chResume <- struct{}{}
	// Don't have to wait for resumeing done.
	return nil
}

func UpdateTouches(touches []Touch) {
	currentUI.m.Lock()
	ox, oy, _, _ := currentUI.screenPaddingImpl()
	s := currentUI.actualScaleImpl()
	currentUI.m.Unlock()
	currentInput.updateTouches(touches, -int(ox/s), -int(oy/s))
}
