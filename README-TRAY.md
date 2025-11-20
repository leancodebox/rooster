# rooster-desktop

```
go install fyne.io/fyne/v2/cmd/fyne@latest # 安装 fyne cmd
fyne package -os darwin   --name rooster-desktop -icon resource/logo.png  roostertray/main.go # mac加入图标打包
fyne package -os linux    --name rooster-desktop -icon resource/logo.png  roostertray/main.go  # linux加入图标打包
fyne package -os windows  --name rooster-desktop -icon resource/logo.png  roostertray/main.go # windows加入图标打包

```
