package main

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/plumbum/go2dpf"
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
			bgImg, err := LoremPixel(w, h)
			if err != nil {
				log.Print(err)
				break
			}

			r := bgImg.Bounds()
			img := go2dpf.NewRGB565Image(bgImg)

			log.Print("Put image to DPF")
			for x := r.Min.X; x < r.Max.X; x += 16 {
				for y := r.Min.Y; y < r.Max.Y; y += 16 {
					chunkRect := image.Rect(
						(x+y) % w, y,
						(x+y) % w + 16, y + 16)
					chunk := img.SubImage(chunkRect).(*go2dpf.ImageRGB565)
					// pp.Println(chunk)
					if err := dpf.Blit(chunk); err != nil {
						log.Fatal(err)
					}
				}
			}
		}
	}

	log.Print("Done")
}

func LoremPixel(w, h int) (image.Image, error) {
	url := fmt.Sprintf("http://lorempixel.com/%d/%d/", w, h)
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
