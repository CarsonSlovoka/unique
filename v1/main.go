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

type Hash string

type File struct {
	Path string
	Info os.FileInfo
}

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

	files := make(map[Hash]File, 0)
	hasherMd5 := md5.New()
	removeFiles := make(map[Hash][]File, 0) // 考慮到想根據條件只保留某一筆重複的資料，例如依據建立日期等等 // 同一個hash之中的所有重複檔案都納入
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
				hashStr := Hash(hex.EncodeToString(hasherMd5.Sum(nil)))
				if firstF, exists := files[hashStr]; exists {
					if removeFiles[hashStr] == nil {
						// 初始化, 並放入第一筆資料
						removeFiles[hashStr] = []File{firstF} // 這一筆為一開始最先找到的檔案
					}
					removeFiles[hashStr] = append(removeFiles[hashStr], File{path, info}) // 當前的檔案也要推入
					log.Printf("重複檔案:%q\n", path)
				} else {
					files[hashStr] = File{path, info}
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
		for _, curFiles := range removeFiles {
			var keep *File
			for _, curF := range curFiles {
				if keep == nil {
					keep = &curF
					continue
				}

				// 只保留創建時間最早的檔案，其他的都刪除
				var removePath string
				if curF.CTime().After(keep.CTime()) {
					// 表示當前檔案較早建立，移除先前的keep檔案
					removePath = keep.Path
					keep = &curF
				} else {
					// 表示這個檔案較晚建立
					removePath = curF.Path
				}
				if err = os.Remove(removePath); err != nil {
					log.Println(err)
					continue
				}
				log.Printf("成功移除:%q\n", removePath)
			}
		}
	}
}
