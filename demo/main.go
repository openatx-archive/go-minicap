package main

import (
	"bytes"
	"fmt"
	"html/template"
	"image"
	"image/jpeg"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/nfnt/resize"
	minicap "github.com/openatx/go-minicap"
)

var (
	imC      <-chan image.Image
	upgrader = websocket.Upgrader{}
)

func test() {
	m, err := minicap.NewService(minicap.Options{Serial: "EP7333W7XB"})
	if err != nil {
		log.Fatal(err)
	}
	err = m.Install()
	if err != nil {
		log.Fatal(err)
	}
	imC, err = m.CaptureFreqFixed(20)
	if err != nil {
		log.Fatal(err)
	}
}

func hIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	tmpl := template.Must(template.New("t").ParseFiles("index.html"))
	tmpl.ExecuteTemplate(w, "index.html", nil)
}

func hImageWs(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade err:", err)
		return
	}
	done := make(chan bool, 1)
	go func() {
		buf := new(bytes.Buffer)
		buf.Reset()
		log.Println("Prepare websocket send", imC)
		for im := range imC {
			select {
			case <-done:
				log.Println("finished")
				return
			default:
			}
			size := im.Bounds().Size()
			newIm := resize.Resize(uint(size.X)/2, 0, im, resize.Lanczos3)
			wr, err := c.NextWriter(websocket.BinaryMessage)
			if err != nil {
				log.Println(err)
				break
			}

			if err := jpeg.Encode(wr, newIm, nil); err != nil {
				break
			}
			wr.Close()
		}
	}()
	for {
		mt, p, err := c.ReadMessage()
		if err != nil {
			log.Println(err)
			done <- true
			break
		}
		log.Println(mt, p, err)
	}
}

func startWebServer(port int) {
	log.Printf("server listern on http://localhost:%d ...", port)

	http.HandleFunc("/", hIndex)
	http.HandleFunc("/ws", hImageWs)
	http.ListenAndServe(fmt.Sprintf(":%v", port), nil)
}

func main() {
	test()
	startWebServer(7000)
}
