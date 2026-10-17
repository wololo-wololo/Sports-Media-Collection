package api

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/stashapp/stash/internal/static"
	"github.com/stashapp/stash/pkg/hash"
	"github.com/stashapp/stash/pkg/logger"
)

type imageBox struct {
	box   fs.FS
	files []string
}

var imageBoxExts = []string{
	".jpg",
	".jpeg",
	".png",
	".gif",
	".svg",
	".webp",
	".avif",
}

func newImageBox(box fs.FS) (*imageBox, error) {
	ret := &imageBox{
		box: box,
	}

	err := fs.WalkDir(box, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		baseName := strings.ToLower(d.Name())
		for _, ext := range imageBoxExts {
			if strings.HasSuffix(baseName, ext) {
				ret.files = append(ret.files, path)
				break
			}
		}

		return nil
	})

	return ret, err
}

func (box *imageBox) GetRandomImageByName(name string) ([]byte, error) {
	files := box.files
	if len(files) == 0 {
		return nil, errors.New("box is empty")
	}

	index := hash.IntFromString(name) % uint64(len(files))
	img, err := box.box.Open(files[index])
	if err != nil {
		return nil, err
	}
	defer img.Close()

	return io.ReadAll(img)
}

var performerBoxCustom *imageBox

func initCustomPerformerImages(customPath string) {
	if customPath != "" {
		logger.Debugf("Loading custom performer images from %s", customPath)
		var err error
		performerBoxCustom, err = newImageBox(os.DirFS(customPath))
		if err != nil {
			logger.Warnf("error loading custom performer images from %s: %v", customPath, err)
		}
	} else {
		performerBoxCustom = nil
	}
}

func getDefaultPerformerImage(name string) []byte {
	// try the custom box first if we have one
	if performerBoxCustom != nil {
		ret, err := performerBoxCustom.GetRandomImageByName(name)
		if err == nil {
			return ret
		}
		logger.Warnf("error loading custom default performer image: %v", err)
	}

	return static.ReadAll(static.DefaultPerformerImage)
}
