/*
* File: minicap_test.go
* Author : bigwavelet
* Description: android minicap test
* Created: 2016-09-13
 */

package minicap

import (
	"testing"
	"time"
)

func TestNewService(t *testing.T) {
	serial := "EP7333W7XB" //your device serial here...
	option := Options{}
	option.Serial = serial
	option.Host = "localhost"
	option.Port = 2333
	option.Adb = "adb"
	_, err := NewService(option)
	if err != nil {
		t.Error("New Service error:" + err.Error())
	} else {
		t.Log("New Service Test Passed.")
	}
}

func TestInstall(t *testing.T) {
	serial := "EP7333W7XB" //your device serial here...
	option := Options{}
	option.Serial = serial
	option.Host = "localhost"
	option.Port = 2333
	option.Adb = "adb"
	m, err := NewService(option)
	if err != nil {
		t.Error("New Service error:" + err.Error())
		return
	}
	err = m.Install()
	if err != nil {
		t.Error("mincap service Install Test error:" + err.Error())
	} else {
		t.Log("Install Test Passed.")
	}
}

func TestIsSupported(t *testing.T) {
	serial := "EP7333W7XB" //your device serial here...
	option := Options{}
	option.Serial = serial
	option.Host = "localhost"
	option.Port = 2333
	option.Adb = "adb"
	m, err := NewService(option)
	if err != nil {
		t.Error("New Service error:" + err.Error())
		return
	}
	err = m.Install()
	if err != nil {
		t.Error("mincap service Install Test error:" + err.Error())
		return
	}
	supported := m.IsSupported()
	if supported {
		t.Log("minicap supported.")
	} else {
		t.Log("minicap not supported.")
	}
}

func TestCaptureAndLastScreenShot(t *testing.T) {
	serial := "EP7333W7XB" //your device serial here...
	option := Options{}
	option.Serial = serial
	option.Host = "localhost"
	option.Port = 2333
	option.Adb = "adb"
	m, err := NewService(option)
	if err != nil {
		t.Error("New Service error:" + err.Error())
		return
	}
	err = m.Install()
	if err != nil {
		t.Error("mincap service Install Test error:" + err.Error())
		return
	}
	supported := m.IsSupported()
	if !supported {
		t.Log("minicap not supported.")
		return
	}
	_, err = m.Capture()
	if err != nil {
		t.Error("Capture Test error:", err.Error())
		return
	} else {
		t.Log("Capture Test passed.")
	}

	time.Sleep(5 * time.Second)
	_, err = m.LastScreenshot()
	if err != nil {
		t.Error("LastScreenshot test error:" + err.Error())
	} else {
		t.Log("LastScreenshot test passed.")
	}
}

func TestScreenshot(t *testing.T) {
	serial := "EP7333W7XB" //your device serial here...
	option := Options{}
	option.Serial = serial
	option.Host = "localhost"
	option.Port = 2333
	option.Adb = "adb"
	m, err := NewService(option)
	if err != nil {
		t.Error("New Service error:" + err.Error())
		return
	}
	err = m.Install()
	if err != nil {
		t.Error("mincap service Install Test error:" + err.Error())
		return
	}
	supported := m.IsSupported()
	if !supported {
		t.Log("minicap not supported.")
		return
	}
	_, err = m.Screenshot()
	if err != nil {
		t.Error("Screenshot Test error:" + err.Error())
	} else {
		t.Log("Screenshot Test Passed.")
	}
}

func TestFps(t *testing.T) {
	serial := "EP7333W7XB" //your device serial here...
	option := Options{}
	option.Serial = serial
	option.Host = "localhost"
	option.Port = 2333
	option.Adb = "adb"
	m, err := NewService(option)
	if err != nil {
		t.Error("New Service error:" + err.Error())
		return
	}
	err = m.Install()
	if err != nil {
		t.Error("mincap service Install Test error:" + err.Error())
		return
	}

	fps, err := m.FPS()
	if err != nil {
		t.Error("FPS Test error:" + err.Error())
	} else {
		t.Log("Fps data:", fps)
		t.Log("FPS Test Passed.")
	}
}
