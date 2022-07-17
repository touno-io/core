package win32

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/lxn/win"
	"golang.org/x/sys/windows"
)

func main() {
	if runtime.GOOS == "windows" {
		imagePath, _ := windows.UTF16PtrFromString(`D:/dvgamerr/Downloads/FUyu7xjaMAMDHFd.jpg`)
		fmt.Println("[+] Changing background now...")
		win.SystemParametersInfo(20, 0, unsafe.Pointer(imagePath), 0x001A)
		// go win.SendMessage(0xFFFF, 0x112, 0xF170, 0x2)
	} else {
		fmt.Println("[x] win32 not supported.")
	}
}
