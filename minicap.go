/*
* File: minicap.go
* Author : bigwavelet
* Description: android minicap service
* Created: 2016-09-13
 */

package minicap

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log"
	"math/rand"
	"net"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
	//	"github.com/pixiv/go-libjpeg/jpeg"  // not work on windows
)

var (
	ErrAlreadyClosed = errors.New("already closed")
	HOST             = "127.0.0.1"
)

type Options struct {
	Serial string
	Port   int
	Adb    string
}

type Service struct {
	d        AdbDevice
	proc     *exec.Cmd
	port     int
	host     string
	r        Rotation
	dispInfo DisplayInfo

	closed    bool
	imageC    chan image.Image
	mu        sync.Mutex
	lastImage image.Image
}

/*
Create Minicap Service
Description:
Serial : device serialno
Port(default: random): minicap service port
Adb(default: adb): adb path
Eg.
	opt := Option{}
	opt.Serial = "aaa"
	service := minicap.NewService(opt)
*/
func NewService(option Options) (s Service, err error) {
	s = Service{}
	s.d, err = newAdbDevice(option.Serial, option.Adb)
	if option.Port == 0 {
		port, err := randPort()
		if err != nil {
			return s, errors.New("port required")
		}
		s.port = port
	} else {
		s.port = option.Port
	}
	s.host = HOST
	s.closed = true
	s.r, err = newRotationService(option)
	if err != nil {
		return
	}
	return
}

/*
Install minicap on device
Eg.
	service := minicap.NewService(opt)
	err := service.Install()
P.s.
	Install function will download files, so keep network connected.
*/
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
Eg.
	service := minicap.NewService(opt)
	supported := service.IsSupported()

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

/*
Uninstall minicap service
Remove minicap on the device
Eg.
	service := minicap.NewService(opt)
	err := service.Uninstall()
*/
func (s *Service) Uninstall() (err error) {
	for _, filename := range []string{"minicap.so", "minicap"} {
		fileExists := s.d.isFileExists("/data/local/tmp/" + filename)
		if fileExists {
			if _, err := s.d.shell("rm /data/local/tmp/" + filename); err != nil {
				return err
			}
		}
	}
	return
}

/*
Get One screenshot
Eg.
	im, err := service.Screenshot()
Todo.
	directly read remote file by goadb
*/
func (s *Service) Screenshot() (im image.Image, err error) {
	if !s.IsSupported() {
		err = errors.New("sorry, minicap not supported")
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
	im, err = jpeg.Decode(fout)
	fout.Close()
	return
}

/*
Capture screen stream based on minicap
Eg.
	imageC, err := service.Capture()
*/
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
	if err = s.startMinicap(s.dispInfo.Orientation); err != nil {
		return
	}
	if err = s.captureMinicap(); err != nil {
		return
	}

	select {
	case <-orienC:
	case <-time.After(500 * time.Millisecond):
		return nil, errors.New("cannot fetch rotation")
	}

	go func() {
		for {
			orientation := <-orienC
			if orientation != s.dispInfo.Orientation {
				s.dispInfo.Orientation = orientation
				if err := s.startMinicap(orientation); err != nil {
					break
				}
				time.Sleep(time.Duration(10+rand.Intn(100)) * time.Millisecond)
			}
		}
	}()
	return s.imageC, nil
}

//start minicap
func (s *Service) startMinicap(orientation int) (err error) {
	if !s.IsSupported() {
		err = errors.New("sorry, minicap not supported")
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
	s.closeMinicap()
	params := fmt.Sprintf("%dx%d@%dx%d/%d", s.dispInfo.Width, s.dispInfo.Height,
		s.dispInfo.Width, s.dispInfo.Height, orientation)
	log.Println(params)
	s.proc = s.d.buildCommand("LD_LIBRARY_PATH=/data/local/tmp", "/data/local/tmp/minicap", "-P", params, "-S")
	if s.proc.Start() != nil {
		return
	}
	time.Sleep(time.Millisecond)
	if _, err = s.d.run("forward", fmt.Sprintf("tcp:%d", s.port), "localabstract:minicap"); err != nil {
		return
	}
	s.closed = false
	return
}

/*
Close Minicap Stream
Eg.
	err := service.Close()
*/
func (s *Service) Close() (err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrAlreadyClosed
	}
	s.closed = true
	close(s.imageC)
	s.closeMinicap()
	s.d.run("forward", "--remove", fmt.Sprintf("tcp:%d", s.port))
	return
}

func (s *Service) closeMinicap() (err error) {
	if s.proc != nil && s.proc.Process != nil {
		s.proc.Process.Signal(syscall.SIGTERM)
	}
	//kill minicap ps on device
	err = s.d.killProc("minicap")
	return
}

/*
Check whether the minicap stream is closed.
Eg.
	closed := service.IsClosed()
*/
func (s *Service) IsClosed() (Closed bool) {
	return s.closed
}

// screen capture
func (s *Service) captureMinicap() (err error) {
	var conn net.Conn
	s.dispInfo, err = s.d.getDisplayInfo()
	if err != nil {
		return
	}
	if err = s.startMinicap(s.dispInfo.Orientation * 90); err != nil {
		return
	}
	s.imageC = make(chan image.Image, 1)
	go func() {
		for {
			conn, err = net.Dial("tcp", fmt.Sprintf("%s:%d", s.host, s.port))
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
				im, err = jpeg.Decode(lr)
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

/*
Get last screen shot from minicap stream if stream is open. Otherwise, call Screenshot function.
Eg.
	im, err := service.LastScreenshot()
*/
func (s *Service) LastScreenshot() (im image.Image, err error) {
	if s.lastImage == nil {
		im, err = s.Screenshot()
		return
	}
	return s.lastImage, nil
}
