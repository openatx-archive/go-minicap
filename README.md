
[![GoDoc](https://godoc.org/github.com/BigWavelet/go-minicap?status.svg)](https://godoc.org/github.com/BigWavelet/go-minicap)

This is a minicap library written based on golang.



## Usage

you can fetch the library by
```shell
go get github.com/BigWavelet/go-minicap
```
then you can use it as follows

```go
package main

import (
	"fmt"
    "log"
	"image/jpeg"
	"os"
	"time"
    "github.com/BigWavelet/go-minicap"
)

func main() {

	serial := "EP7333W7XB" //your serial here...
	option := Options{}
	option.Serial = serial
	m, err := NewService(option)
	if err != nil {
		log.Fatal(err)
		return
	}

	err = m.Install()
	if err != nil {
		log.Println(err.Error())
		return
	}
	log.Println(m.IsSupported())

	log.Println("start to capture")
	_, err := m.Capture()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("capture finished..")
	log.Println("Start to close minicap")
	err = m.Close()
	if err != nil {
		log.Fatal(err)
	}

}

```