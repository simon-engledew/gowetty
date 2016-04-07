package main

import (
  "github.com/googollee/go-socket.io"
  "github.com/kr/pty"
  "github.com/GeertJohan/go.rice"
  "log"
  "net/http"
  "path"
  "os"
  "os/exec"
  "syscall"
  "unsafe"
)

type winsize struct {
  ws_row    uint16
  ws_col    uint16
  ws_xpixel uint16
  ws_ypixel uint16
}

func setwindowrect(ws *winsize, fd uintptr) error {
  _, _, errno := syscall.Syscall(
    syscall.SYS_IOCTL,
    fd,
    syscall.TIOCSWINSZ,
    uintptr(unsafe.Pointer(ws)),
  )
  if errno != 0 {
    return syscall.Errno(errno)
  }
  return nil
}

func main() {
  log.SetFlags(log.Lshortfile)

  if len(os.Args) < 2 {
    log.Fatal("usage: ", path.Base(os.Args[0]), " <command> [<args> ...]")
  }

  server, err := socketio.NewServer(nil)
  if err != nil {
    log.Fatal(err)
  }

  server.On("connection", func(so socketio.Socket) {
    log.Print("CLIENT CONNECTED")

    c := exec.Command(os.Args[1], os.Args[2:]...)
    f, err := pty.Start(c)
    if err != nil {
      panic(err)
    }
    buffer := make([]byte, 9216)

    go func() {
      c.Wait()
      log.Print("TERMINATED")
    }()

    go func() {
      for true {
        n, err := f.Read(buffer)
        if err != nil {
          log.Print(err)
          break
        }
        so.Emit("output", string(buffer[:n]))
      }
    }()

    so.On("resize", func(data map[string]uint16) {
      ws := new(winsize)
      ws.ws_row = data["row"]
      ws.ws_col = data["col"]
      setwindowrect(ws, f.Fd())
    })

    so.On("input", func(data string) {
      f.Write([]byte(data))
    })

    so.On("disconnection", func() {
      c.Process.Kill()
    })

  })

  server.On("error", func(so socketio.Socket, err error) {
    log.Println("error:", err)
  })

  http.Handle("/wetty/socket.io/", server)
  // http.Handle("/", http.FileServer(http.Dir("webroot")))
  http.Handle("/", http.FileServer(rice.MustFindBox("webroot").HTTPBox()))

  log.Print("Listening on :3000")
  log.Fatal(http.ListenAndServe(":3000", nil))
}
