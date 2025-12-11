# Image Compressor

A simple, fast, and efficient desktop application for compressing images, built with Go and the Fyne toolkit.

![Screenshot](Screenshot%202025-12-11%20at%2022.53.40.png) 

## Features

- **Cross-Platform:** Works on macOS, Windows, and Linux.
- **Batch Processing:** Compress multiple images from files and folders at once.
- **Multiple Image Formats:** Supports JPEG, PNG, WEBP, BMP, and TIFF.
- **Flexible Compression:**
  - Set a target file size in KB.
  - Specify maximum width and height for resizing.
  - Defaults to 85% JPEG quality if no target size is set.
- **EXIF Handling:** Automatically corrects image orientation based on EXIF data.
- **No Overwrites:** Prevents accidental file loss by creating unique filenames for compressed images (e.g., `image (1).jpg`).
- **Image Preview:** See a preview of the selected image before compressing.

## How to Use

1.  **Add Images:**
    - Click "Add Files/Folders" to select individual image files or entire folders containing images.
2.  **Select Output Folder:**
    - Click "Browse..." to choose where the compressed images will be saved.
3.  **Set Compression Options (Optional):**
    - **Target Size:** Enter a target size in kilobytes (KB). The app will try to get as close as possible to this size. If left at 0, a default JPEG quality of 85% will be used.
    - **Max Dimensions:** Set the maximum width and height in pixels to resize the image while maintaining aspect ratio.
4.  **Start Compression:**
    - Click "Start Compress" to begin the process. The progress bar will show the status.

## Building from Source

To build and run this application from source, you need to have Go and the Fyne dependencies installed.

### Prerequisites

- **Go:** Version 1.17 or later.
- **Fyne Dependencies:** Follow the instructions on the [Fyne website](https://developer.fyne.io/started/) to set up your system for Fyne development. This usually involves installing a C compiler (like GCC) and other graphics libraries.

### Build Steps

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/your-username/Image-compressor-golang-desktop-app.git
    cd Image-compressor-golang-desktop-app
    ```

2.  **Tidy dependencies:**
    ```bash
    go mod tidy
    ```

3.  **Build and run the application:**
    ```bash
    go run main.go
    ```

4.  **Build a standalone executable:**
    ```bash
    go build -o image-compressor
    ```

## Dependencies

This project relies on the following Go libraries:

- [fyne.io/fyne/v2](https://github.com/fyne-io/fyne): A cross-platform GUI toolkit for Go.
- [github.com/disintegration/imaging](https://github.com/disintegration/imaging): An image processing library for Go.
- [github.com/rwcarlsen/goexif](https://github.com/rwcarlsen/goexif): A library for reading EXIF data from images.
