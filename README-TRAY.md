# rooster-desktop

```
go run  roostertray/main.go
go install fyne.io/fyne/v2/cmd/fyne@latest # 安装 fyne cmd
cd roostertray && fyne package -os darwin  --name rooster-desktop -icon ../resource/logo.png   # mac加入图标打包
cd roostertray && fyne package -os linux   --name rooster-desktop -icon ../resource/logo.png   # linux加入图标打包
cd roostertray && fyne package -os windows --name rooster-desktop -icon ../resource/logo.png   # windows加入图标打包

```
