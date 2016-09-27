
[![GoDoc](https://godoc.org/github.com/openatx/go-minicap?status.svg)](https://godoc.org/github.com/openatx/go-minicap)

This is a [minicap](https://github.com/openstf/minicap) library written based on golang.



## Usage
```sh
go get -v github.com/openatx/go-minicap
```

Code example

```go
package main

import (
	"fmt"
    "log"
	"image/jpeg"
	"os"
	"time"
    "github.com/openatx/go-minicap"
)

func main() {
	serial := "EP7333W7XB" //your serial here...
	option := minicap.Options{}
	option.Serial = serial
	m, err := minicap.NewService(option)
	if err != nil {
		log.Fatal(err)
	}

	err = m.Install()
	if err != nil {
		log.Fatal(err)
	}
	log.Println(m.IsSupported())

	log.Println("start to capture")
	imageC, err := m.Capture()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("capture finished..")

	for im := range(imageC) {
		log.Println(im)
	}

	log.Println("Start to close minicap")
	err = m.Close()
	if err != nil {
		log.Fatal(err)
	}
}
```

## demo

you can run the [demo](/demo/main.go)

then you can visit http://127.0.0.1:5678 to see the screen in real-time. Like,

![](demo/demo.png)

## LICENSE
Under [MIT](LICENSE)