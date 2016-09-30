package minicap

import (
	"image"
	"time"
)

//Sampling minicap with limited sampling rate
func LimitedSampling(imgC <-chan image.Image, freq int) <-chan image.Image {
	imgLmtC := make(chan image.Image, 1)
	interval := time.Duration(1000 / freq)
	go func() {
		tick := time.NewTicker(interval)
		defer tick.Stop()
		var lastImage image.Image
		var ok = true
		for ok {
			var img image.Image
			select {
			case img, ok = <-imgC:
				if !ok {
					break
				}
				lastImage = img
			case <-tick.C:
				if lastImage != nil {
					imgLmtC <- lastImage
					lastImage = nil
				}
			}
		}
	}()

	return imgLmtC
}

//Sampling minicap with fixed sampling rate
func FixedSampling(imgC <-chan image.Image, freq int) <-chan image.Image {
	imgFxdC := make(chan image.Image, 1)
	interval := time.Duration(1000 / freq)
	go func() {

		tick := time.NewTicker(interval)
		defer tick.Stop()
		var lastImage image.Image
		var ok = true
		for ok {
			var img image.Image
			select {
			case img, ok = <-imgC:
				if !ok {
					break
				}
				lastImage = img
			case <-tick.C:
				if lastImage != nil {
					imgFxdC <- lastImage
				}
			}
		}
	}()
	return imgFxdC
}
