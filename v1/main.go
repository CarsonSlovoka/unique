package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"flag"
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
	WkDir     string `json:"wkDir"`
	Suffixes  []string
	Condition string
}

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", ".unique.json", "請輸入設定檔的路徑")
	flag.Parse()

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
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		for _, suffix := range cfg.Suffixes { // .png, .jpg, ...
			if suffix == "*" || filepath.Ext(path) == suffix {
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
						removeFiles[hashStr] = []File{firstF}         // 這一筆為一開始最先找到的檔案
						absPathFirstF, _ := filepath.Abs(firstF.Path) // 顯示絕對路徑，避免工作路徑沒有而砍錯檔案
						log.Printf("重複檔案:%q\n", absPathFirstF)
					}
					removeFiles[hashStr] = append(removeFiles[hashStr], File{path, info}) // 當前的檔案也要推入
					absPath, _ := filepath.Abs(path)
					log.Printf("重複檔案:%q\n", absPath)
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

	if len(removeFiles) == 0 {
		log.Println("沒有任何重複項目！")
		return
	}

	user32dll := w32.NewUser32DLL(w32.PNMessageBox)
	response, _ := user32dll.MessageBox(0, "是否要移除所有重複檔案", "確認", w32.MB_YESNO)
	if response == w32.IDYES {
		removeByCondition(removeFiles, &cfg.Condition)
	}
}

// // 在眾多的重複項裡頭，依據條件，只保留一個
func removeByCondition(files map[Hash][]File, condition *string) {
	isNeedUpdateKeep := func(curF *File, keep *File) bool {
		switch *condition {
		case "cTime":
			if curF.CTime().Before(keep.CTime()) {
				return true // 表示curF的建立日期比keep還要早
			}
			return false
		case "len":
			fallthrough
		default:
			if len(curF.Path) < len(keep.Path) {
				return true // 表示當前的檔案路徑較短
			}
			return false
		}
	}

	var err error
	for _, curFiles := range files {
		var keep *File
		for i, curF := range curFiles {
			if keep == nil {
				// keep = &curF // 錯誤curF會異動，這樣keep也會跟著跑
				keep = &curFiles[i]
				continue
			}

			var removePath string
			if isNeedUpdateKeep(&curF, keep) {
				removePath = keep.Path
				keep = &curF
			} else {
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
