package main

import (
	_ "embed"
	"github.com/leancodebox/rooster/jobmanager"
	"github.com/leancodebox/rooster/jobmanagerserver"
	"log/slog"
	"os"
	"os/signal"
)

func init() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
}

func main() {
	//if _, err := os.Stat("jobConfig.json"); os.IsNotExist(err) {
	//	fmt.Println("当前目录下不存在jobConfig.json文件")
	//
	//	// 询问是否生成该文件
	//	fmt.Print("是否生成jobConfig.json文件？(yes/no): ")
	//	var answer string
	//	_, err := fmt.Scanln(&answer)
	//	if err != nil {
	//		fmt.Println("无法读取输入，错误：", err)
	//		return
	//	}
	//
	//	if answer == "yes" {
	//		err = os.WriteFile("jobConfig.json", resource.GetJobConfigDefault(), 0644)
	//		if err != nil {
	//			fmt.Println("无法写入文件，错误：", err)
	//			return
	//		}
	//		fmt.Println("jobConfig.json文件已生成并写入内容。请调整配置后再次启动")
	//	} else {
	//		fmt.Println("请补充 jobConfig.json 后再次启动程序")
	//	}
	//	return
	//}
	//fileData, err := os.ReadFile("jobConfig.json")
	//if err != nil {
	//	slog.Error(err.Error())
	//	return
	//}

	err := jobmanager.RegByUserConfig()
	if err != nil {
		slog.Error(err.Error())
		return
	}

	// jobmanager.Reg(fileData)
	jobmanagerserver.ServeRun()
	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	<-quit
	jobmanagerserver.ServeStop()
	slog.Info("bye~~👋👋")
}
