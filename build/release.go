/*
wkDir: 本腳本路徑
go run release.go
*/

package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"syscall"
	textTemplate "text/template"
)

type Config struct {
	Version string
	AppName string
	PkgName string
	LdFlags string
	ZipPsw  string
	Info    struct { // 此為用於填充詳細資料所用
		Desc         string
		ProductName  string
		RequireAdmin bool
		Copyright    string
		Lang         string
	}
}

var cfg Config

func init() {
	f, err := os.Open("config.json")
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = f.Close()
	}()
	decoder := json.NewDecoder(f)
	if err = decoder.Decode(&cfg); err != nil {
		panic(err)
	}
}

// TextColor 可以更改console的顏色
func TextColor(fr, fg, fb, br, bg, bb int) func(msg string) string {
	return func(msg string) string {
		return fmt.Sprintf("\u001B[48;2;%d;%d;%dm\u001B[38;2;%d;%d;%dm%s\u001B[0m",
			br, bg, bb,
			fr, fg, fb,
			msg,
		)
	}
}

var (
	YText func(msg string) string
)

func init() {
	YText = TextColor(0, 0, 0, 255, 255, 0)
}

const (
	TEnUS string = "en-us"
	TZhTW        = "zh-tw"
	TZhCn        = "zh-cn"
	TJaJp        = "ja-jp"
)

type AppInfoContext struct {
	Version      string
	ExeName      string
	Desc         string
	ProductName  string
	Copyright    string
	Translation  string
	RequireAdmin bool
}

// BuildAllTmpl 提供app.manifest, resources.rc等樣版路徑，建立出相對應的檔案
// 如果要刪除所產生出來的產物，可以呼叫deferFunc
func BuildAllTmpl(manifestPath, resourcesPath string, appInfoCtx *AppInfoContext) (err error, deferFunc func() error) {
	if !strings.HasSuffix(manifestPath, ".gotmpl") || !strings.HasSuffix(resourcesPath, ".gotmpl") {
		return fmt.Errorf("please make sure the file suffix is %q", ".gotmpl"), nil
	}

	funcMap := map[string]any{
		"ternary": func(condition bool, trueVal, falseVal any) any {
			if condition {
				return trueVal
			}
			return falseVal
		},
		"replaceAll": func(s, old, new string) string {
			return strings.ReplaceAll(s, old, new)
		},
		"dict": func(values ...any) (map[string]any, error) {
			if len(values)%2 != 0 {
				return nil, errors.New("parameters must be even")
			}
			dict := make(map[string]any)
			var key, val any
			for {
				key, val, values = values[0], values[1], values[2:]
				switch reflect.ValueOf(key).Kind() {
				case reflect.String:
					dict[key.(string)] = val
				default:
					return nil, errors.New(`type must equal to "string"`)
				}
				if len(values) == 0 {
					break
				}
			}
			return dict, nil
		},
		// 自動填補到4碼的版號
		"makeValidVersion": func(version string) string {
			validVersion := version
			for i := 0; i < 3-strings.Count(version, "."); i++ {
				validVersion += ".0"
			}
			return validVersion
		},
	}
	tmplPaths := make([]string, 0) // 紀錄產生出來的tmp檔案，最後要移除
	for _, tmplPath := range []string{manifestPath, resourcesPath} {
		outFilePath := tmplPath[:len(tmplPath)-7] // app.manifest.gotmpl => app.manifest
		outF, err := os.OpenFile(outFilePath, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			return err, nil
		}
		tmplPaths = append(tmplPaths, outFilePath)
		t := textTemplate.Must(
			textTemplate.New(filepath.Base(tmplPath)).Funcs(funcMap).ParseFiles(tmplPath),
		)
		if err = t.Execute(outF, appInfoCtx); err != nil {
			return err, nil
		}
		if err = outF.Close(); err != nil {
			return err, nil
		}
	}
	return nil, func() error {
		for _, curF := range tmplPaths {
			if err := os.Remove(curF); err != nil {
				return err
			}
		}
		return nil
	}
}

func getCmd(name string, arg ...string) *exec.Cmd {
	cmd := exec.Command(name, arg...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	log.Println(YText(strings.Join(cmd.Args, " ")))
	return cmd
}

func cmdRun(name string, arg ...string) error {
	cmd := getCmd(name, arg...)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

/* 不要用syso來加圖標，可能會導致執行檔無法運作
func ManifestToSyso(output string) error {
	return cmdRun("rsrc", "-manifest", "app.manifest", "-o", output, "-ico", "app.ico")
}
*/

// BuildMain 主程式
func BuildMain(wkDir, output string) error {
	args := []string{"build", "-ldflags", cfg.LdFlags, "-o", output,
		"--pkgdir", "..", // go.mod位於工作目錄的哪裡
	}
	cmd := getCmd("go", args...)
	cmd.Dir = wkDir
	return cmd.Run()
}

// Rc2Res "resource.rc" to "resource.res"
func Rc2Res(output string) error {
	return cmdRun("ResourceHacker",
		"-open", "resources.rc", "-save", output,
		"-action", "compile",
		"-log", "CONSOLE",
	)
}

func AddVersionInf(exePath, resPath string) error {
	return cmdRun(
		"ResourceHacker", "-open", exePath, "-save", exePath,
		"-resource", resPath,
		"-action", "addoverwrite",
		"-mask", "VersionInf", "-log", "CONSOLE",
	)
}

func AddIcon(exePath, iconPath string) error {
	return cmdRun(
		"ResourceHacker", "-open", exePath, "-save", exePath,
		"-resource", iconPath,
		"-action", "addoverwrite", "-mask", "ICONGROUP,MAINICON,", // 注意icon的mask後面要有","不然會失敗
		"-log", "CONSOLE",
	)
}

// 只抓檔案忽略資料夾
func getFileOnly(dirEntry []os.DirEntry, acceptSuffix []string) (files []os.DirEntry) {
	for _, f := range dirEntry {
		if f.IsDir() {
			continue
		}
		ok := true
		if len(acceptSuffix) > 0 {
			ok = false
			for _, suffix := range acceptSuffix {
				if strings.HasSuffix(strings.ToLower(f.Name()), suffix) {
					ok = true
					break
				}
			}
		}
		if ok {
			files = append(files, f)
		}
	}
	return files
}

func ZipSource(psw string) (zipFilePath string, err error) {
	zipName := fmt.Sprintf("%s_%s_%s_%s.zip", cfg.AppName, runtime.GOOS, runtime.GOARCH, cfg.Version)
	// 確保輸出目錄存在 (已存在不影響，不存在就新建)
	if err = os.MkdirAll("temp", os.ModePerm); err != nil {
		return "", err
	}
	zipFilePath = filepath.Join("temp/" + zipName)
	fz, err := os.Create(zipFilePath)
	if err != nil {
		return "", err
	}
	defer func() { // defer LIFO 後進先出的特性，因此第一個defer才是最尾聲的內容
		err = fz.Close() // 注意如果每個的名稱都用f，那麼到了defer之後的f有可能已經和您預想的f不同了

		// Add Psw
		if psw != "" { // 必須要在zipWriter關閉之後才可以加密，否則會遇到: 程序無法存取檔案，因為檔案正由另一個程序使用。
			cmd := getCmd("7z",
				"a", zipName[:len(zipName)-3]+"7z", // 輸出的檔案名稱,注意副檔名需要為7z
				zipName, // 要加什麼項目到此zip去，若用"*"表示這個資料夾內的所有項目都要加入
				// 用以下的這種方式，產出的7z或多一層temp，所以直接切換cmd.Dir比較快
				// "a", "temp/"+zipName[:len(zipName)-3]+"7z",
				"-p"+psw,
				"-mhe=on", // 隱藏目錄結構讓其不可見
			)
			cmd.Dir = "./temp"
			err = cmd.Run()
		}
	}()

	zipWriter := zip.NewWriter(fz)
	defer func() {
		fmt.Println("closing zip archive...")
		err = zipWriter.Close()
	}()

	for _, d := range []struct {
		srcPath string
		outPath string
	}{
		{"../unique/.unique.json", "example-unique.json"},

		{fmt.Sprintf("./%s.exe", cfg.AppName), cfg.AppName + ".exe"},
	} {
		log.Printf("opening %q...\n", d.srcPath)
		f, err := os.Open(d.srcPath)
		if err != nil {
			return "", err
		}

		log.Printf("writing %q to archive...\n", d.outPath)
		w, err := zipWriter.Create(d.outPath)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(w, f); err != nil {
			return "", err
		}
		_ = f.Close()
	}

	// merge release-note
	{
		buf := bytes.NewBuffer([]byte(""))
		dirEntrySlice, err := os.ReadDir("doc/release-note/")
		if err != nil {
			return "", fmt.Errorf("read release-note dir error. %w", err)
		}

		files := getFileOnly(dirEntrySlice, nil)

		sort.Slice(files, func(i, j int) bool { // 依照版號大小排序(Desc)
			return files[i].Name() > files[j].Name()
		})
		for _, curDirEntry := range files {
			contents, err := os.ReadFile(fmt.Sprintf("doc/release-note/%s", curDirEntry.Name()))
			if err != nil {
				return "", err
			}
			buf.Write([]byte(fmt.Sprintf("%s\n\n", contents)))
		}

		// 寫入zip
		w, err := zipWriter.Create("release-note.md")
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(w, buf); err != nil {
			return "", err
		}
	}
	return zipFilePath, nil
}

func main() {
	if err, removeAllTmplFunc := BuildAllTmpl(
		"app.manifest.gotmpl", "resources.rc.gotmpl",
		&AppInfoContext{
			cfg.Version,
			cfg.AppName,
			cfg.Info.Desc,
			cfg.Info.ProductName,
			cfg.Info.Copyright,
			cfg.Info.Lang,
			cfg.Info.RequireAdmin}); err != nil {
		log.Fatal(err)
	} else {
		defer func() {
			if err = removeAllTmplFunc(); err != nil {
				log.Fatal(err)
			}
		}()
	}

	wkDir, _ := filepath.Abs("../unique")
	outputExePath := cfg.AppName + ".exe"
	if err := BuildMain(wkDir, filepath.Join("../build/", outputExePath)); err != nil {
		log.Fatal(err)
	}

	res := "resources.res"
	if err := Rc2Res(res); err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = os.Remove(res)
	}()

	if err := AddVersionInf(outputExePath, res); err != nil {
		log.Fatal(err)
	}

	if err := AddIcon(outputExePath, "app.ico"); err != nil {
		log.Fatal(err)
	}

	zipFilePath, err := ZipSource(cfg.ZipPsw)
	if err != nil {
		log.Fatal(err)
	}

	// 顯示zip檔案的sha256雜湊值
	hasher256 := sha256.New()
	{
		bs, err := os.ReadFile(zipFilePath)
		if err != nil {
			log.Fatal(err)
		}
		hasher256.Write(bs)
		fmt.Println("sha256:" + hex.EncodeToString(hasher256.Sum(nil)))
	}

	// 開啟輸出的資料夾(temp)
	{
		tempPath, _ := filepath.Abs("./temp")
		if runtime.GOOS == "darwin" {
			_ = exec.Command("open", tempPath).Start()
		} else {
			_ = exec.Command("explorer", tempPath).Start()
		}
	}
}
