/*
* File: core.go
* Author : bigwavelet
* Description: core file
* Created: 2016-08-26
 */

package minicap

import (
	"io"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"unicode"
)

func strip(str string) (result string) {
	str = strings.TrimSpace(str)
	result = strings.TrimRightFunc(str, unicode.IsSpace)
	return
}

func splitLines(str string) (result []string) {
	tmp := strings.Replace(str, "\r\n", "\n", -1)
	tmp = strings.Replace(str, "\r", "", -1)
	result = strings.Split(tmp, "\n")
	return
}

func downloadFile(fileName string, url string) (err error) {
	fout, err := os.Create(fileName)
	if err != nil {
		return
	}
	defer fout.Close()
	response, err := http.Get(url)
	if err != nil {
		return
	}
	defer response.Body.Close()
	_, err = io.Copy(fout, response.Body)
	if err != nil {
		return
	}
	return
}

func randSeq(n int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func randPort() (int, error) {
	var letters = []rune("12345")
	b := make([]rune, 5)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	port, err := strconv.Atoi(string(b))
	return port, err
}
