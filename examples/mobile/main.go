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

// Package mobile is an example of Ebiten for mobiles (Android).
// You can `gomobile bind` this package but not `gomobile build`.
package mobile

import (
	_ "image/jpeg"
	"log"
	"math"

	"github.com/hajimehoshi/ebiten"
	"github.com/hajimehoshi/ebiten/examples/common"
	"github.com/hajimehoshi/ebiten/mobile"
)

const (
	screenWidth  = 320
	screenHeight = 240
)

var (
	count        int
	gophersImage *ebiten.Image
)

func update(screen *ebiten.Image) error {
	count++
	w, h := gophersImage.Size()
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-float64(w)/2, -float64(h)/2)
	op.GeoM.Rotate(float64(count%360) * 2 * math.Pi / 360)
	op.GeoM.Translate(screenWidth/2, screenHeight/2)
	if err := screen.DrawImage(gophersImage, op); err != nil {
		return err
	}
	return nil
}

func Start() {
	var err error
	gophersImage, _, err = common.AssetImage("gophers.jpg", ebiten.FilterNearest)
	if err != nil {
		log.Fatal(err)
	}
	mobile.Start(update, screenWidth, screenHeight, 2, "Mobile (Ebiten Demo)")
}