package minicap

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	"io"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"
	//	"github.com/pixiv/go-libjpeg/jpeg"  // not work on windows
)

var (
	ErrAlreadyClosed = errors.New("already closed")
)

type Options struct {
	Serial string
	Adb    string
}

type Service struct {
	AdbPort int
	AdbHost string

	lforwardPort int // local forward port
	d            AdbDevice
	r            Rotation
	dispInfo     DisplayInfo

	closed    bool
	imageC    chan image.Image
	mu        sync.Mutex
	lastImage image.Image
}

func NewService(opt Options) (s *Service, err error) {
	s = &Service{
		AdbPort: 5037,
		AdbHost: "localhost",
		closed:  true,
	}
	s.d, err = newAdbDevice(opt.Serial, opt.Adb)
	if err != nil {
		return
	}

	s.r, err = newRotationService(opt)
	if err != nil {
		return
	}
	return
}

// Install minicap and minicap.so to /data/local/tmp
// files downloaded from github.com/openstf/minicap
func (s *Service) Install() (err error) {
	err = s.r.install()
	if err != nil {
		return
	}
	abi, err := s.d.getProp("ro.product.cpu.abi")
	if err != nil {
		return
	}
	sdk, err := s.d.getProp("ro.build.version.sdk")
	if err != nil {
		return
	}
	for _, filename := range []string{"minicap.so", "minicap"} {
		isExists := s.d.isFileExists("/data/local/tmp/" + filename)
		if isExists {
			continue
		}
		// download file from github
		var url string
		if filename == "minicap.so" {
			url = "https://github.com/openstf/stf/raw/master/vendor/minicap/shared/android-" + sdk + "/" + abi + "/minicap.so"
		} else {
			url = "https://github.com/openstf/stf/raw/master/vendor/minicap/bin/" + abi + "/minicap"
		}
		fName := "/data/local/tmp/" + filename
		err = s.r.download(fName, url)
		if err != nil {
			return
		}
	}
	return
}

/*
Check whether minicap is supported on the device
For more information, see: https://github.com/openstf/minicap
*/
func (s *Service) IsSupported() bool {
	fileExists := s.d.isFileExists("/data/local/tmp/minicap")
	if !fileExists {
		err := s.Install()
		if err != nil {
			return false
		}
	}
	out, err := s.d.shell("LD_LIBRARY_PATH=/data/local/tmp /data/local/tmp/minicap -i")
	if err != nil {
		return false
	}
	supported := strings.Contains(out, "height") && strings.Contains(out, "width")
	return supported
}

// Remove minicap and minicap.so from device
func (s *Service) Uninstall() (err error) {
	for _, filename := range []string{"minicap.so", "minicap"} {
		if _, err := s.d.shell("rm", "-f", "/data/local/tmp/"+filename); err != nil {
			return err
		}
	}
	return nil
}

// Take screenshot
// If minicap in on, the return the last recent image
func (s *Service) Screenshot() (im image.Image, err error) {
	if !s.IsSupported() {
		err = errors.New("minicap not supported") // FIXME(ssx): maybe need to fallback to screencap
		return
	}
	dispInfo, err := s.d.getDisplayInfo()
	if err != nil {
		return
	}
	if dispInfo.Width > dispInfo.Height {
		dispInfo.Width, dispInfo.Height = dispInfo.Height, dispInfo.Width
	}
	params := fmt.Sprintf("%dx%d@%dx%d/%d", dispInfo.Width, dispInfo.Height,
		dispInfo.Width, dispInfo.Height, dispInfo.Orientation*90)
	fName := randSeq(10)
	fName = fmt.Sprintf("go_%v.jpg", fName)
	cmd := fmt.Sprintf("LD_LIBRARY_PATH=/data/local/tmp /data/local/tmp/minicap -n minicap -P %v -s > /data/local/tmp/%v", params, fName)
	_, err = s.d.shell(cmd)
	if err != nil {
		return
	}
	fout, err := s.d.Device.OpenRead("/data/local/tmp/" + fName)
	if err != nil {
		return
	}
	im, _, err = image.Decode(fout)
	fout.Close()
	return
}

// Capture screen stream based on minicap
func (s *Service) Capture() (imageC <-chan image.Image, err error) {
	err = s.r.start()
	if err != nil {
		return
	}
	orienC, err := s.r.watch()
	if err != nil {
		return
	}
	s.dispInfo, err = s.d.getDisplayInfo()
	if err != nil {
		return
	}
	if err = s.runMinicap(s.dispInfo.Orientation); err != nil {
		return
	}
	if err = s.startReadFromSocket(); err != nil {
		return
	}

	// TODO(ssx): too slow here
	select {
	case <-orienC:
	case <-time.After(time.Second):
		return nil, errors.New("cannot fetch rotation")
	}

	go func() {
		for {
			orientation := <-orienC
			if orientation != s.dispInfo.Orientation {
				s.dispInfo.Orientation = orientation
				if err := s.runMinicap(orientation); err != nil {
					break
				}
				time.Sleep(time.Duration(10+rand.Intn(100)) * time.Millisecond)
			}
		}
	}()
	return s.imageC, nil
}

//Sampling minicap with fixed sampling rate
func (s *Service) FixedSampling(imC <-chan image.Image, freq int) <-chan image.Image {
	imgFxdC := make(chan image.Image, 1)
	go func() {
		interval := int64(1e9 / freq)
		for {
			start := time.Now()
			select {
			case im := <-imC:
				imgFxdC <- im
			case <-time.After(time.Millisecond):
				im, err := s.LastScreenshot()
				if err != nil {
					imgFxdC <- im
				}
			}
			duration := time.Since(start).Nanoseconds()
			time.Sleep(time.Duration(interval-duration) * time.Nanosecond)
		}
	}()
	return imgFxdC
}

// Start Minicap until the minicap started
func (s *Service) runMinicap(orientation int) (err error) {
	if !s.IsSupported() {
		err = errors.New("minicap not supported")
		return
	}
	if s.dispInfo.Height == 0 {
		s.dispInfo, err = s.d.getDisplayInfo()
		if err != nil {
			return
		}
	}
	if s.dispInfo.Width > s.dispInfo.Height {
		s.dispInfo.Width, s.dispInfo.Height = s.dispInfo.Height, s.dispInfo.Width
	}
	s.close()
	params := fmt.Sprintf("%dx%d@%dx%d/%d", s.dispInfo.Width, s.dispInfo.Height,
		s.dispInfo.Width, s.dispInfo.Height, orientation)
	cmd := s.d.buildCommand("LD_LIBRARY_PATH=/data/local/tmp", "/data/local/tmp/minicap", "-P", params, "-S")
	if err = cmd.Start(); err != nil {
		return
	}
	time.Sleep(time.Millisecond) // ?
	if s.lforwardPort == 0 {
		s.lforwardPort, err = freePort()
		if err != nil {
			return
		}
	}
	if _, err = s.d.run("forward", fmt.Sprintf("tcp:%d", s.lforwardPort), "localabstract:minicap"); err != nil {
		return
	}
	s.closed = false
	return
}

// Close Minicap Service
func (s *Service) Close() (err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrAlreadyClosed
	}
	s.closed = true
	close(s.imageC)
	s.close()
	s.d.run("forward", "--remove", fmt.Sprintf("tcp:%d", s.lforwardPort))
	return
}

func (s *Service) close() (err error) {
	return s.d.killProc("minicap")
}

// Check whether the minicap stream is closed.
func (s *Service) IsClosed() (Closed bool) {
	return s.closed
}

// read image from socket
func (s *Service) startReadFromSocket() (err error) {
	var conn net.Conn
	s.dispInfo, err = s.d.getDisplayInfo()
	if err != nil {
		return
	}
	err = s.runMinicap(s.dispInfo.Orientation * 90)
	if err != nil {
		return
	}
	s.imageC = make(chan image.Image, 1)
	go func() {
		for {
			conn, err = net.Dial("tcp", fmt.Sprintf("%s:%d", s.AdbHost, s.lforwardPort))
			if err != nil {
				continue
			}
			var pid, rw, rh, vw, vh uint32
			var version uint8
			var unused uint8
			var orientation uint8
			binRead := func(data interface{}) error {
				if err != nil {
					return err
				}
				err = binary.Read(conn, binary.LittleEndian, data)
				return err
			}
			binRead(&version)
			binRead(&unused)
			binRead(&pid)
			binRead(&rw)
			binRead(&rh)
			binRead(&vw)
			binRead(&vh)
			binRead(&orientation)
			binRead(&unused)
			if err != nil {
				continue
			}
			bufrd := bufio.NewReader(conn) // Do not put it into for loop
			for {
				var size uint32
				if err = binRead(&size); err != nil {
					break
				}
				lr := &io.LimitedReader{bufrd, int64(size)}
				var im image.Image
				im, _, err = image.Decode(lr)
				if err != nil {
					break
				}
				s.mu.Lock()
				if s.closed {
					break
				}
				s.lastImage = im
				select {
				case s.imageC <- im:
				default:
				}
				s.mu.Unlock()
			}
			conn.Close()
		}
	}()
	return nil
}

// Return last screenshot from minicap
// if minicap is closed, use Screenshot() instead
func (s *Service) LastScreenshot() (im image.Image, err error) {
	if s.lastImage == nil || s.IsClosed() {
		im, err = s.Screenshot()
		return
	}
	return s.lastImage, nil
}
