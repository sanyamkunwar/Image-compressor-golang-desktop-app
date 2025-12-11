package main

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/disintegration/imaging"
	"github.com/rwcarlsen/goexif/exif"
)

//
// Single-threaded (main-thread) image compressor
// - No goroutines touching UI
// - Processing happens synchronously when user clicks Start
// - Compiles across Fyne versions because no RunOnMain/CallOnMain/Invoke used
//

// Prevent overwrite: if "name.jpg" exists → use "name (1).jpg", etc.
func uniqueOutputPath(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}

	dir := filepath.Dir(path)
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	name := base[:len(base)-len(ext)]

	counter := 1
	for {
		newName := fmt.Sprintf("%s (%d)%s", name, counter, ext)
		newPath := filepath.Join(dir, newName)
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			return newPath
		}
		counter++
	}
}

// Load image and correct EXIF rotation
func loadImageApplyEXIF(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	img, err := imaging.Decode(f)
	f.Close()
	if err != nil {
		return nil, err
	}

	// Read EXIF again for orientation
	ef, err := os.Open(path)
	if err != nil {
		return img, nil
	}
	ex, err := exif.Decode(ef)
	ef.Close()
	if err != nil {
		return img, nil // no EXIF → fine
	}

	orientTag, err := ex.Get(exif.Orientation)
	if err != nil {
		return img, nil
	}
	orient, err := orientTag.Int(0)
	if err != nil {
		return img, nil
	}

	switch orient {
	case 3:
		img = imaging.Rotate180(img)
	case 6:
		img = imaging.Rotate270(img)
	case 8:
		img = imaging.Rotate90(img)
	}

	return img, nil
}

// Encode to JPEG with a given quality
func encodeJPEGBytes(img image.Image, q int) ([]byte, error) {
	buf := &bytes.Buffer{}
	err := jpeg.Encode(buf, img, &jpeg.Options{Quality: q})
	return buf.Bytes(), err
}

// Binary-search quality for target size
func findQualityForTarget(img image.Image, targetBytes int) ([]byte, int, error) {
	lo, hi := 10, 95
	var best []byte
	var bestQ int

	for lo <= hi {
		mid := (lo + hi) / 2
		data, err := encodeJPEGBytes(img, mid)
		if err != nil {
			return nil, 0, err
		}
		if len(data) <= targetBytes {
			best = data
			bestQ = mid
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}

	if best == nil {
		data, err := encodeJPEGBytes(img, 10)
		return data, 10, err
	}

	return best, bestQ, nil
}

func listImages(root string) ([]string, error) {
	var files []string
	exts := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".webp": true,
		".bmp": true, ".tiff": true,
	}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && exts[filepath.Ext(path)] {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

// processImageSync does the actual work synchronously on the main thread.
func processImageSync(inPath, outPath string, targetKB, maxW, maxH int) (string, error) {
	img, err := loadImageApplyEXIF(inPath)
	if err != nil {
		return "", fmt.Errorf("load failed: %v", err)
	}

	// resize
	if maxW > 0 || maxH > 0 {
		img = imaging.Fit(img, maxW, maxH, imaging.Lanczos)
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return "", fmt.Errorf("mkdir failed: %v", err)
	}

	if targetKB <= 0 {
		// save jpeg with quality 85
		if err := imaging.Save(img, outPath, imaging.JPEGQuality(85)); err != nil {
			return "", fmt.Errorf("save failed: %v", err)
		}
		info, _ := os.Stat(outPath)
		return fmt.Sprintf("OK %s -> %s (%dKB)", inPath, outPath, info.Size()/1024), nil
	}

	// target mode
	targetBytes := targetKB * 1024
	data, q, err := findQualityForTarget(img, targetBytes)
	if err != nil {
		return "", fmt.Errorf("compress failed: %v", err)
	}
	if err := ioutil.WriteFile(outPath, data, 0644); err != nil {
		return "", fmt.Errorf("write failed: %v", err)
	}
	return fmt.Sprintf("OK %s -> %s (q=%d, %dKB)", inPath, outPath, q, len(data)/1024), nil
}

func main() {
	a := app.NewWithID("com.sanyam.imagecompressor")
	w := a.NewWindow("Image Compressor (macOS) — Simple")
	w.Resize(fyne.NewSize(1000, 650))

	var items []string
	selectedIndex := -1

	// List widget
	list := widget.NewList(
		func() int { return len(items) },
		func() fyne.CanvasObject { return widget.NewLabel("template") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			if i >= 0 && i < len(items) {
				o.(*widget.Label).SetText(filepath.Base(items[i]))
			}
		},
	)

	preview := canvas.NewText("No preview selected", nil)
	previewContainer := container.NewCenter(preview)

	addBtn := widget.NewButton("Add Files/Folders", func() {
		fd := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
			if err != nil || r == nil {
				return
			}
			path := r.URI().Path()
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				imgs, err := listImages(path)
				if err == nil {
					items = append(items, imgs...)
					list.Refresh()
				}
			} else {
				items = append(items, path)
				list.Refresh()
			}
		}, w)
		fd.Show()
	})

	outEntry := widget.NewEntry()
	outEntry.SetPlaceHolder("Select output folder (use Browse...)")

	browseOutBtn := widget.NewButton("Browse...", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			outEntry.SetText(uri.Path())
		}, w)
	})

	targetEntry := widget.NewEntry()
	targetEntry.SetPlaceHolder("Target size KB (0 = normal JPEG)")
	widthEntry := widget.NewEntry()
	widthEntry.SetPlaceHolder("Max width (px)")
	heightEntry := widget.NewEntry()
	heightEntry.SetPlaceHolder("Max height (px)")

	progressBar := widget.NewProgressBar()
	progressBar.Hide()
	statusLabel := widget.NewLabel("Idle")

	startBtn := widget.NewButton("Start Compress (blocking)", func() {
		if len(items) == 0 {
			dialog.ShowInformation("No Input", "Add files or folders first.", w)
			return
		}
		outFolder := outEntry.Text
		if outFolder == "" {
			dialog.ShowInformation("No Output", "Select output folder.", w)
			return
		}

		// parse options
		targetKB := 0
		fmt.Sscanf(targetEntry.Text, "%d", &targetKB)
		maxW := 0
		fmt.Sscanf(widthEntry.Text, "%d", &maxW)
		maxH := 0
		fmt.Sscanf(heightEntry.Text, "%d", &maxH)

		// expand items
		var images []string
		for _, p := range items {
			if info, err := os.Stat(p); err == nil && info.IsDir() {
				imgs, err := listImages(p)
				if err == nil {
					images = append(images, imgs...)
				}
			} else {
				images = append(images, p)
			}
		}
		if len(images) == 0 {
			dialog.ShowInformation("No Images", "No image files found.", w)
			return
		}

		// Prepare UI
		progressBar.SetValue(0)
		progressBar.Show()
		statusLabel.SetText("Starting...")

		total := len(images)
		for i, f := range images {
			// compute output path and ensure unique
			base := filepath.Base(f)
			name := base[:len(base)-len(filepath.Ext(base))]
			outPath := filepath.Join(outFolder, name+".jpg")
			outPath = uniqueOutputPath(outPath)

			msg, err := processImageSync(f, outPath, targetKB, maxW, maxH)
			if err != nil {
				statusLabel.SetText("Error: " + err.Error())
				// continue processing other images
			} else {
				statusLabel.SetText(msg)
			}
			progressBar.SetValue(float64(i+1) / float64(total))
		}

		statusLabel.SetText("Done")
	})

	removeBtn := widget.NewButton("Remove Selected", func() {
		if selectedIndex >= 0 && selectedIndex < len(items) {
			items = append(items[:selectedIndex], items[selectedIndex+1:]...)
			list.Unselect(widget.ListItemID(selectedIndex))
			selectedIndex = -1
			list.Refresh()
			preview.Text = "No preview selected"
			previewContainer.Refresh()
		}
	})

	clearBtn := widget.NewButton("Clear All", func() {
		items = nil
		selectedIndex = -1
		list.Refresh()
		preview.Text = "No preview selected"
		previewContainer.Refresh()
	})

	// preview on select
	list.OnSelected = func(id widget.ListItemID) {
		if id < 0 || int(id) >= len(items) {
			selectedIndex = -1
			return
		}
		selectedIndex = int(id)
		path := items[id]
		img := canvas.NewImageFromFile(path)
		img.FillMode = canvas.ImageFillContain
		img.SetMinSize(fyne.NewSize(400, 400))
		previewContainer.Objects = []fyne.CanvasObject{img}
		previewContainer.Refresh()
	}

	left := container.NewBorder(
		container.NewVBox(widget.NewLabel("Files to compress"), widget.NewLabel("Click an item to preview")),
		nil, nil, nil,
		container.NewVScroll(list),
	)

	opts := container.NewVBox(
		widget.NewLabel("Preview"),
		previewContainer,
		widget.NewSeparator(),
		container.NewGridWithColumns(2, widget.NewLabel("Output folder:"), outEntry),
		container.NewHBox(browseOutBtn),
		targetEntry,
		container.NewHBox(widthEntry, heightEntry),
		startBtn,
		progressBar,
		statusLabel,
		widget.NewSeparator(),
		container.NewHBox(removeBtn, clearBtn, addBtn),
	)

	content := container.NewHSplit(left, opts)
	content.Offset = 0.35
	w.SetContent(content)
	w.ShowAndRun()
}
