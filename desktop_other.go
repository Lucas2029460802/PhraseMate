//go:build !windows && !darwin

package main

import "log"

func runDesktop(url string) {
	log.Printf("当前系统未使用桌面窗口模式，请在浏览器打开: %s", url)
	log.Println("或设置 PHRASEMATE_WEB=1 明确使用浏览器模式")
	waitForShutdown()
}

func runWebOnly(url string) {
	log.Printf("浏览器模式，请打开: %s", url)
	log.Println("提示: 浏览器模式没有系统级置顶悬浮窗")
}
