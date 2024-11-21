package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

func updateStatus(fname string) (int, string) {
	j := map[string]string{"name": "./outputs/" + fname}
	jsonValue, _ := json.Marshal(j)

	resp, err := http.Post("http://localhost:4999/status", "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		fmt.Println(err)
		return 0, ""
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return 0, ""
	}
	var data struct {
		Path   string `json:"path"`
		Status int    `json:"status"`
	}
	err = json.Unmarshal(body, &data)
	if err != nil {
		fmt.Println(err)
		return 0, ""
	}
	fmt.Println(data.Path)
	fmt.Println(data.Status)
	return data.Status, data.Path
}

func countFiles(directory string, extensions []string) int {
	var count int
	files, err := os.ReadDir(directory)
	if err != nil {
		fmt.Println(err)
		return 0
	}
	for _, file := range files {
		path := filepath.Join(directory, file.Name())
		ext := filepath.Ext(path)
		for _, e := range extensions {
			if ext == "."+e {
				count++
			}
		}
	}
	return count
}

func bootServer() {
	http.Post("http://localhost:4999/end", "application/json", bytes.NewBuffer([]byte{}))
	cmd := exec.Command("./sdgoenv/Scripts/python.exe", "./backend/backend.py")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		panic(err)
	}
	j := map[string]string{"model": "models/sd3.5_medium.safetensors"}
	jsonValue, _ := json.Marshal(j)

	time.Sleep(10 * time.Second)
	http.Post("http://localhost:4999/start", "application/json", bytes.NewBuffer(jsonValue))
	fmt.Println("Server started successfully!")
}

func generateImage(p, pn string, cfg float64, seed, width, height, steps int) {
	name := fmt.Sprintf("sdgo-%06d-%d.png", countFiles("./outputs", []string{"png", "jpg", "webp"}), imageCounter)
	values := map[string]interface{}{"prompt": p,
		"neg_prompt": pn,
		"width":      width,
		"height":     height,
		"seed":       seed,
		"steps":      steps,
		"cfg":        cfg,
		"name":       name,
	}

	jsonValue, _ := json.Marshal(values)

	http.Post("http://localhost:4999/generate", "application/json", bytes.NewBuffer(jsonValue))
	imageProgress[name] = 1
}

func newImageTab(parent *container.AppTabs, window fyne.Window, imagePath string, width int, height int, seed, steps int, cfg float64, prompt, neg_prompt string) *container.TabItem {
	closeButton := widget.NewButton("Close", func() {})
	fmt.Println("Image:", imagePath)
	if imagePath == "" {
		tab := container.NewBorder(nil, container.NewVBox(widget.NewLabel(fmt.Sprintf("Seed: %s", strconv.Itoa(seed))),
			widget.NewLabel(fmt.Sprintf("Width: %d", width)),
			widget.NewLabel(fmt.Sprintf("Height: %d", height)),
			container.NewHBox(widget.NewButton("View", func() {
				// Open the image in an external viewer
				cmd := exec.Command("explorer.exe", imagePath)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Start(); err != nil {
					panic(err)
				}
			}),
				widget.NewButton("Save", func() {
					// Save the image as a PNG
				}),
				widget.NewButton("Save Config", func() {
					// Save the generation settings to a text file
				}),
				closeButton,
			)), nil, nil, widget.NewLabel("Generating..."))

		generateImage(prompt, neg_prompt, cfg, seed, width, height, steps)
		tabTab := container.NewTabItem(fmt.Sprintf("Image #%d", imageCounter), tab)
		imageCounter++

		closeButton.OnTapped = func() {
			parent.Remove(parent.CurrentTab())
		}
		return tabTab
	} else {
		imageCanv := canvas.NewImageFromFile(imagePath)
		imageCanv.FillMode = canvas.ImageFillContain
		tab := container.NewBorder(nil, container.NewVBox(widget.NewLabel(fmt.Sprintf("Seed: %s", strconv.Itoa(seed))),
			widget.NewLabel(fmt.Sprintf("Width: %d", width)),
			widget.NewLabel(fmt.Sprintf("Height: %d", height)),
			container.NewHBox(widget.NewButton("View", func() {
				fmt.Println("Viewing:", imagePath)
				absolute, _ := filepath.Abs(imagePath)
				cmd := exec.Command("explorer.exe", absolute)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Start(); err != nil {
					panic(err)
				}
			}),
				widget.NewButton("Save", func() {
					// Save the image as a PNG
					fd := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
						//defer writer.Close()
						if strings.HasSuffix(strings.ToLower(writer.URI().Path()), "png") {
							b, _ := os.ReadFile(imagePath)
							//_, err := writer.Write(b)
							err2 := os.WriteFile(writer.URI().Path(), b, 0644)
							if err != nil {
								dialog.NewError(err, window)
							}
							if err2 != nil {
								dialog.NewError(err2, window)
							}
						}
					}, window)
					fd.Show()
				}),
				widget.NewButton("Save Config", func() {
					// Save the generation settings to a text file
				}),
				closeButton,
			)), nil, nil, imageCanv)
		tabTab := container.NewTabItem("Image", tab)
		closeButton.OnTapped = func() {
			parent.Remove(parent.CurrentTab())
		}
		imageCanv.Refresh()
		return tabTab
	}
}

var imageCounter int
var imageProgress map[string]int

func init() {
	// initizlize global variables
	imageCounter = 1
	imageProgress = make(map[string]int)
}

func main() {
	myApp := app.New()
	myWindow := myApp.NewWindow("Stable Diffusion UI")
	promptContainer := container.NewVBox(
		widget.NewLabel("Prompt:"),
		widget.NewMultiLineEntry(),
		widget.NewLabel("Negative Prompt:"),
		widget.NewMultiLineEntry(),
	)
	widthSlider := widget.NewSlider(64, 2048)
	widthSlider.Step = 64
	heightSlider := widget.NewSlider(64, 2048)
	heightSlider.Step = 64
	seedEntry := widget.NewEntry()
	seedEntry.SetText("-1")
	stepSlider := widget.NewSlider(1, 100)
	sizeContainer := container.NewVBox(
		widget.NewLabel("Seed:"),
		seedEntry,
		widget.NewLabel("Width:"),
		widthSlider,
		widget.NewLabel("Height:"),
		heightSlider,
		widget.NewLabel("Steps:"),
		stepSlider,
	)
	cfgSlider := widget.NewSlider(0, 14)
	cfgContainer := container.NewVBox(
		widget.NewLabel("CFG:"),
		cfgSlider,
		widget.NewLabel("Info:"),
		widget.NewLabel(""),
	)
	cfgContainer.Objects[1].(*widget.Slider).Step = 0.1
	cfgSlider.SetValue(4.5)
	tabs := container.NewAppTabs()
	generateContainer := container.NewVBox(
		widget.NewButton("Generate", func() {
			imageName := fmt.Sprintf("sdgo-%06d-%d.png", countFiles("./outputs", []string{"png", "jpg", "webp"}), imageCounter)
			width, height := int(sizeContainer.Objects[3].(*widget.Slider).Value), int(sizeContainer.Objects[5].(*widget.Slider).Value)
			seed := rand.Intn(1000000)
			parsed, _ := strconv.ParseInt(seedEntry.Text, 10, 64)
			if parsed > 0 {
				seed = int(parsed)
			}
			tab := newImageTab(tabs, myWindow, "", width, height, seed, int(stepSlider.Value), cfgSlider.Value, promptContainer.Objects[1].(*widget.Entry).Text, promptContainer.Objects[3].(*widget.Entry).Text)
			tabs.Append(tab)
			go func() {
				status, path := 0, ""
				for range time.Tick(time.Second / 4) {
					status, path = updateStatus(imageName)
					if status == 2 || status == 1 {
						if status == 2 {
							path = "./outputs/" + imageName
							fmt.Println("ImageName:", path)
						}
						tab2 := newImageTab(tabs, myWindow, path, width, height, seed, int(stepSlider.Value), cfgSlider.Value, promptContainer.Objects[1].(*widget.Entry).Text, promptContainer.Objects[3].(*widget.Entry).Text)
						tab.Content = tab2.Content
						tabs.Refresh()
						//tab = tab2

						if status == 2 {
							break
						}
					}
				}
			}()
		}),
	)
	mainContainer := container.NewVBox(
		promptContainer,
		sizeContainer,
		cfgContainer,
		generateContainer,
	)
	//sizeContainer.Objects[5].(*widget.Slider).Bind(binding.BindFloat(cfgContainer.Objects[2].(*widget.Label)))
	sizeContainer.Objects[3].(*widget.Slider).OnChanged = func(value float64) {
		megapixels := sizeContainer.Objects[3].(*widget.Slider).Value * sizeContainer.Objects[5].(*widget.Slider).Value / 1_000_000
		cfgContainer.Objects[3].(*widget.Label).SetText(fmt.Sprintf("Steps: %d\nCFG: %.1f\nWidth: %d\nHeight: %d\nMegapixels: %.2f", int(stepSlider.Value), cfgSlider.Value, int(sizeContainer.Objects[3].(*widget.Slider).Value), int(sizeContainer.Objects[5].(*widget.Slider).Value), megapixels))
	}
	sizeContainer.Objects[5].(*widget.Slider).OnChanged = sizeContainer.Objects[3].(*widget.Slider).OnChanged
	cfgSlider.OnChanged = sizeContainer.Objects[3].(*widget.Slider).OnChanged
	stepSlider.OnChanged = cfgSlider.OnChanged
	tabs.Append(container.NewTabItem("Generation", mainContainer))
	myWindow.SetContent(tabs)
	myWindow.Resize(fyne.NewSize(800, 600))
	widthSlider.SetValue(1024)
	heightSlider.SetValue(1024)
	stepSlider.SetValue(30)
	widthSlider.OnChanged(1024)
	heightSlider.OnChanged(1024)
	bootServer()
	icon, _ := fyne.LoadResourceFromPath("sdgo-logo.png")
	myApp.SetIcon(icon)
	myWindow.SetIcon(icon)
	myWindow.ShowAndRun()
	fmt.Println("Shutting Down!")
	http.Get("http://localhost:4999/end")
}
