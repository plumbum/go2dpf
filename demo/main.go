package main

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"net/http"
	"os"

	"github.com/plumbum/go2dpf"
	"os/signal"
	"syscall"
)

func main() {

	log.SetOutput(os.Stderr)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	log.Print("Open DPF")
	dpf, err := go2dpf.OpenDpf()
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		dpf.Close()
		log.Print("USB closed")
	}()

	w, h, err := dpf.GetDimensions()
	if err != nil {
		log.Fatal("LcdParams: ", err)
	}
	fmt.Printf("Got LCD dimensions: %dx%d\n", w, h)

	if err := dpf.Brightness(7); err != nil {
		log.Fatal(err)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

ForExit:
	for {
		select {
		case sig := <-sigs:
			log.Print("Got signal: ", sig)
			break ForExit
		default:

			log.Print("Load image")
			bgImg, err := LoremPixel()
			if err != nil {
				log.Print(err)
				break
			}

			log.Print("Convert image")
			r := bgImg.Bounds()
			img := go2dpf.NewRGB565(r)
			for y := r.Min.Y; y < r.Max.Y; y++ {
				for x := r.Min.X; x < r.Max.X; x++ {
					img.Set(x, y, bgImg.At(x, y))
				}
			}

			log.Print("Put image to DPF")
			err = dpf.Blit(img)
			if err != nil {
				log.Fatal(err)
			}
		}
	}

	log.Print("Done")
}

func LoremPixel() (image.Image, error) {
	url := "http://lorempixel.com/240/320/"
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Status code: %d (%s)", resp.StatusCode, resp.Status)
	}
	log.Print("Content-Type: ", resp.Header.Get("Content-Type"))

	img, name, err := image.Decode(resp.Body)
	if err != nil {
		return nil, err
	}
	log.Printf("Image format: %s [%dx%d]", name, img.Bounds().Dx(), img.Bounds().Dy())
	return img, nil
}
