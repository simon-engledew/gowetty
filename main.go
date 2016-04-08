package main

import (
  "github.com/googollee/go-socket.io"
  "github.com/kr/pty"
  "github.com/GeertJohan/go.rice"
  "gopkg.in/alecthomas/kingpin.v2"
  "log"
  "fmt"
  "net/http"
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

func Resize(pty *os.File, rows uint16, cols uint16) error {
  ws := winsize {
    ws_row: rows,
    ws_col: cols,
  }
  _, _, errno := syscall.Syscall(
    syscall.SYS_IOCTL,
    pty.Fd(),
    syscall.TIOCSWINSZ,
    uintptr(unsafe.Pointer(&ws)),
  )
  if errno != 0 {
    return syscall.Errno(errno)
  }
  return nil
}

var (
  port = kingpin.Flag("port", "Port to run web server on.").Short('p').Default("3000").Int()
  command = kingpin.Arg("command", "Command to run").Required().String()
  args = kingpin.Arg("args", "Args to pass to command").Strings()
)

func main() {
  log.SetFlags(log.Lshortfile)

  kingpin.Parse()

  server, err := socketio.NewServer(nil)
  if err != nil {
    log.Fatal(err)
  }

  server.On("connection", func(so socketio.Socket) {
    log.Print("CLIENT CONNECTED")

    c := exec.Command(*command, *args...)
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
      Resize(f, data["row"], data["col"])
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
  http.Handle("/", http.FileServer(rice.MustFindBox("webroot").HTTPBox()))

  log.Print(fmt.Sprintf("Listening on :%d", *port))
  log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))
}
