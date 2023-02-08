package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"github.com/CarsonSlovoka/go-pkg/v2/w32"
	"io/fs"
	"log"
	"os"
	"path/filepath"
)

type Config struct {
	WkDir    string `json:"wkDir"`
	Suffixes []string
}

func main() {
	configPath := ".unique.json"
	if _, err := os.Stat(configPath); err != nil {
		log.Fatal(err)
	}
	bytes, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatal(err)
	}

	var cfg Config
	if err = json.Unmarshal(bytes, &cfg); err != nil {
		log.Fatal(err)
	}

	files := make(map[string]string, 0) // [Hash]File
	hasherMd5 := md5.New()
	removeFiles := make([]string, 0)
	err = filepath.Walk(cfg.WkDir, func(path string, info fs.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		for _, suffix := range cfg.Suffixes { // .png, .jpg, ...
			if filepath.Ext(path) == suffix {
				bytes, err = os.ReadFile(path)
				if err != nil {
					log.Printf("讀取檔案失敗: %s\n", err)
					continue
				}
				hasherMd5.Write(bytes)
				hashStr := hex.EncodeToString(hasherMd5.Sum(nil))
				if _, exists := files[hashStr]; exists {
					removeFiles = append(removeFiles, path)
					log.Printf("重複檔案:%q\n", path)
				} else {
					files[hashStr] = ""
				}
				hasherMd5.Reset() // 如果要重複用，就要重製
			}
		}
		return nil
	})
	if err != nil {
		log.Fatal(err) // 工作路徑錯誤
	}

	user32dll := w32.NewUser32DLL(w32.PNMessageBox)
	response, _ := user32dll.MessageBox(0, "是否要移除所有重複檔案", "確認", w32.MB_YESNO)
	if response == w32.IDYES {
		for _, fPath := range removeFiles {
			if err = os.Remove(fPath); err != nil {
				log.Println(err)
				continue
			}
			log.Printf("成功移除:%q\n", fPath)
		}
	}
}
