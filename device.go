/*
* File: device.go
* Author : bigwavelet
* Description: android device interface
* Created: 2016-08-26
 */

package minicap

import (
	"errors"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	adb "github.com/zach-klippenstein/goadb"
)

type AdbDevice struct {
	Serial  string
	AdbPath string
	*adb.Adb
	*adb.Device
}

type DisplayInfo struct {
	Width       int `json:"width"`
	Height      int `json:"height"`
	Orientation int `json:"orientation"`
}

func newAdbDevice(serial, AdbPath string) (d AdbDevice, err error) {
	if serial == "" {
		err = errors.New("serial cannot be empty")
		return
	}
	d.Serial = serial
	if AdbPath == "" {
		d.AdbPath = "adb"
	} else {
		d.AdbPath = AdbPath
	}
	d.Adb, err = adb.NewWithConfig(adb.ServerConfig{
		Port: 5037,
	})
	d.Adb.StartServer()
	d.Device = d.Adb.Device(adb.DeviceWithSerial(serial))
	return
}

func (d *AdbDevice) shell(cmds ...string) (out string, err error) {
	args := []string{"-s", d.Serial, "shell"}
	cmds = append(cmds, ";", "echo", ":$?")
	args = append(args, cmds...)
	output, err := exec.Command(d.AdbPath, args...).Output()
	if err != nil {
		return
	}
	outStr := string(output)
	idx := strings.LastIndexByte(outStr, ':')
	statusCode := outStr[idx+1:]
	out = outStr[:idx]
	if strip(statusCode) != "0" {
		return out, fmt.Errorf("adb shell error: adb %v", args)
	}
	return
}

func (d *AdbDevice) buildCommand(cmds ...string) (out *exec.Cmd) {
	args := []string{}
	args = append(args, "-s", d.Serial, "shell")
	args = append(args, cmds...)
	return exec.Command(d.AdbPath, args...)
}

func (d *AdbDevice) run(cmds ...string) (out string, err error) {
	args := []string{}
	args = append(args, "-s", d.Serial)
	args = append(args, cmds...)
	output, err := exec.Command(d.AdbPath, args...).Output()
	if err != nil {
		return
	}
	out = string(output)
	return
}

func (d *AdbDevice) getProp(key string) (result string, err error) {
	out, err := d.shell("getprop", key)
	if err != nil {
		return
	}
	result = strip(out)
	return
}

func (d *AdbDevice) isFileExists(filename string) bool {
	/*  // Stat takes too long, almost 2 sec
	_, err := d.Device.Stat(filename)
	if err != nil {
		return false
	}
	return true
	*/
	_, err := d.shell("test", "-f", filename)
	if err != nil {
		return false
	}
	return true
}

func (d *AdbDevice) getDisplayInfo() (info DisplayInfo, err error) {
	out, err := d.shell("dumpsys display")
	if err != nil {
		return
	}
	lines := splitLines(string(out))
	patten := regexp.MustCompile(`.*DisplayViewport{valid=true,.*orientation=(\d+),.*deviceWidth=(\d+), deviceHeight=(\d+).*`)
	for _, line := range lines {
		m := patten.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		info.Orientation, err = strconv.Atoi(m[1])
		if err != nil {
			break
		}
		info.Orientation = info.Orientation * 90
		info.Width, err = strconv.Atoi(m[2])
		if err != nil {
			break
		}
		info.Height, err = strconv.Atoi(m[3])
		if err != nil {
			break
		}

		return
	}
	log.Println(info)
	// TODO(ssx): use some other method
	// info.Orientation = 0
	// info.Width = 720
	// info.Height = 1280
	return
}

func (d *AdbDevice) getPackageList() (plist []string, err error) {
	out, err := d.shell("pm list packages")
	if err != nil {
		return
	}
	plist = splitLines(out)
	for i := 0; i < len(plist); i++ {
		plist[i] = strings.Replace(plist[i], "\r", "", -1)
		plist[i] = strings.Replace(plist[i], "\n", "", -1)
		plist[i] = strip(plist[i])
	}
	return
}

func (d *AdbDevice) killProc(psName string) (err error) {
	out, err := d.shell("ps")
	if err != nil {
		return
	}
	fields := strings.Split(strip(out), "\n")
	if len(fields) > 1 {
		var idxPs int
		for idx, val := range strings.Fields(fields[0]) {
			if val == "PID" {
				idxPs = idx
				break
			}
		}
		for _, val := range fields[1:] {
			field := strings.Fields(val)
			if strings.Contains(val, psName) {
				pid := field[idxPs]
				_, err := d.shell("kill", "-9", pid)
				if err != nil {
					return err
				}
			}
		}

	}
	return
}
