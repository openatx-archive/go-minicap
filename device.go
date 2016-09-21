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
}

type DisplayInfo struct {
	Width       int `json:"width"`
	Height      int `json:"height"`
	Density     int `json:"density"`
	Orientation int `json:"orientation"`
}

//new device client

func NewAdbDevice(serial, AdbPath string) (d AdbDevice, err error) {
	if len(strip(serial)) == 0 {
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
	return
}

//adb shell
func (d *AdbDevice) shell(cmds ...string) (out string, err error) {
	args := []string{}
	args = append(args, "-s", d.Serial, "shell")
	args = append(args, cmds...)
	args = append(args, ";echo GO_MINICAP_TAG:$?")
	output, err := exec.Command(d.AdbPath, args...).Output()
	fields := strings.Split(string(output), "GO_MINICAP_TAG:")
	out = fields[0]
	if strip(fields[1]) != "0" {
		return out, errors.New("adb shell error.")
	}
	return
}

func (d *AdbDevice) buildCommand(cmds ...string) (out *exec.Cmd) {
	args := []string{}
	args = append(args, "-s", d.Serial, "shell")
	args = append(args, cmds...)
	return exec.Command(d.AdbPath, args...)
}

func (d *AdbDevice) shellNowait(cmds ...string) (out *exec.Cmd, err error) {
	args := []string{}
	args = append(args, "-s", d.Serial, "shell")
	args = append(args, cmds...)
	cmd := exec.Command(d.AdbPath, args...)
	err = cmd.Start()
	if err != nil {
		return
	}
	out = cmd
	return
}

//adb command

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

func (d *AdbDevice) runNowait(cmds ...string) (out *exec.Cmd, err error) {
	args := []string{}
	args = append(args, "-s", d.Serial)
	args = append(args, cmds...)
	cmd := exec.Command(d.AdbPath, args...)
	err = cmd.Start()
	if err != nil {
		return
	}
	out = cmd
	return
}

func (d *AdbDevice) getProp(key string) (result string, err error) {
	out, err := d.shell("getprop " + key)
	if err != nil {
		return
	}
	result = strip(out)
	return
}

//file check
func (d *AdbDevice) isFileExisted(filename string) (bool, error) {
	out, err := d.shell("ls " + filename)
	if err != nil {
		return false, err
	}
	if strings.Contains(string(out), "No such file or directory") {
		return false, nil
	} else {
		return true, nil
	}
}

//get display info
func (d *AdbDevice) getDisplayInfo() (info DisplayInfo, err error) {
	out, err := d.shell("dumpsys display")
	if err != nil {
		return
	}
	lines := splitLines(string(out))
	patten := regexp.MustCompile(`.*PhysicalDisplayInfo{(?P<width>\d+) x (?P<height>\d+), .*, density (?P<density>[\d.]+).*`)
	for _, line := range lines {
		if !patten.MatchString(line) {
			continue
		}
		m := patten.FindStringSubmatch(line)
		if len(m) >= 4 {
			width, err := strconv.Atoi(m[1])
			if err == nil {
				info.Width = width
			}
			height, err := strconv.Atoi(m[2])
			if err == nil {
				info.Height = height
			}
			density, err := strconv.Atoi(m[3])
			if err == nil {
				info.Density = density
			}
		}
	}
	patten = regexp.MustCompile(`orientation=(\d+)`)
	out, err = d.shell("dumpsys SurfaceFlinger")
	if info.Height > info.Width {
		info.Orientation = 0
	} else {
		info.Orientation = 1
	}
	if err != nil || !patten.MatchString(string(out)) {
		return
	}
	m := patten.FindStringSubmatch(string(out))
	if len(m) >= 2 {
		orientation, err := strconv.Atoi(m[1])
		if err != nil {
			return info, err
		}
		info.Orientation = orientation
	}
	return
}

//get package list
func (d *AdbDevice) getPackageList() (plist []string, err error) {
	out, err := d.shell("pm list packages")
	if err != nil {
		return
	}
	plist = splitLines(out)
	for i := 0; i < len(plist); i++ {
		plist[i] = strings.Replace(plist[i], "\r", "", -1)
		plist[i] = strings.Replace(plist[i], "\n", "", -1)
		plist[i] = strings.Replace(plist[i], " ", "", -1)
	}
	return
}

func (d *AdbDevice) killPs(psName string) (err error) {
	out, err := d.shell("ps")
	if err != nil {
		return
	}
	fields := strings.Split(strip(out), "\n")
	if len(fields) > 1 {
		var idxPs, idxName int
		for idx, val := range strings.Fields(fields[0]) {
			if val == "PID" {
				idxPs = idx
				break
			}
		}
		for idx, val := range strings.Fields(fields[0]) {
			if val == "NAME" {
				idxName = idx
				break
			}
		}
		for _, val := range fields[1:] {
			field := strings.Fields(val)
			if strings.Contains(field[idxName+1], psName) {
				pid := field[idxPs]
				_, err := d.shell(fmt.Sprintf("kill -9 %v", pid))
				if err != nil {
					return err
				}
			}
		}

	}
	return
}
