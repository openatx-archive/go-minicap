package minicap

import (
	"image"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLimitedSampling(t *testing.T) {
	imgC := make(chan image.Image, 2)
	rgba := image.NewRGBA(image.Rect(0, 0, 50, 50))
	imgC <- image.NewRGBA(image.Rect(0, 0, 20, 20))
	imgC <- rgba

	assert := assert.New(t)
	limgC := LimitedSampling(imgC, 10) // 100 frame/s
	select {
	case img := <-limgC:
		assert.Equal(rgba, img, "should be the same image")
	case <-time.After(10 * time.Millisecond):
		t.Fatal("No image get from limgC")
	}

	select {
	case <-limgC:
		t.Fatal("No image should be here, because of sampling limited")
	case <-time.After(10 * time.Millisecond):
		t.Log("should be no image, good")
	}

}
