package main

import (
	"fmt"
	"os"
	"path/filepath"

	"phrasemate/internal/brandicon"
)

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	ico := filepath.Join(root, "PhraseMate.ico")
	if err := brandicon.WriteICO(ico, brandicon.RenderRounded(256)); err != nil {
		panic(err)
	}
	fmt.Println("wrote", ico)

	assets := filepath.Join(root, "assets", "PhraseMate.ico")
	if err := os.MkdirAll(filepath.Dir(assets), 0o755); err == nil {
		if err := brandicon.WriteICO(assets, brandicon.RenderRounded(256)); err != nil {
			panic(err)
		}
		fmt.Println("wrote", assets)
	}

	web := filepath.Join(root, "web", "static", "img", "icon.png")
	if err := os.MkdirAll(filepath.Dir(web), 0o755); err == nil {
		src := filepath.Join(root, "internal", "brandicon", "icon.png")
		data, err := os.ReadFile(src)
		if err != nil {
			panic(err)
		}
		if err := os.WriteFile(web, data, 0o644); err != nil {
			panic(err)
		}
		fmt.Println("wrote", web)
	}
}
