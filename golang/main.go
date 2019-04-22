package main

import (
	"ChatClient/golang/protobuf"
	pb "ChatClient/golang/protobuf"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/gorilla/websocket"
	"github.com/jroimartin/gocui"
)

var show *gocui.View

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func getInput(g *gocui.Gui, v *gocui.View) error {
	buf := v.ViewBuffer()
	if buf == "\n" {
		return nil
	}
	for range buf {
		v.EditDelete(true)
	}
	//fmt.Fprint(show, buf)
	req := MessageReq(buf)
	conn.WriteMessage(websocket.TextMessage, req)

	return nil
}

func layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	if v, err := g.SetView("main", 0, maxY/2+2, maxX-3, maxY-1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Editable = true
		v.Wrap = true
		v.Frame = false
		v.Autoscroll = true
		if _, err := g.SetCurrentView("main"); err != nil {
			return err
		}
	}
	if v2, err := g.SetView("show", 0, 0, maxX-3, maxY/2-1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		show = v2
		v2.Wrap = true
		v2.Frame = false
		v2.Autoscroll = true
	}
	if v3, err := g.SetView("line", 0, maxY/2, maxX-3, maxY/2+1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v3.Frame = true
	}

	return nil
}

var name string
var conn *websocket.Conn
var guiInstance *gocui.Gui

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Please enter the username as command line parameter")
		return
	}
	name = os.Args[1] // first command line parameter, ...

	g, err := gocui.NewGui(gocui.OutputNormal)
	guiInstance = g
	if err != nil {
		log.Panicln(err)
	}

	g.Cursor = false
	g.Mouse = false

	g.SetManagerFunc(layout)

	if err := g.SetKeybinding("main", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		log.Panicln(err)
	}
	if err := g.SetKeybinding("main", gocui.KeyEnter, gocui.ModNone, getInput); err != nil {
		log.Panicln(err)
	}

	// websocket

	roots := loadCA("cert.pem")

	d := websocket.Dialer{TLSClientConfig: &tls.Config{RootCAs: roots}}
	tmp, _, err := d.Dial("wss://127.0.0.1:9090/", nil)
	conn = tmp

	go Worker()

	if err != nil {
		log.Fatal("dial:", err)
	}
	defer conn.Close()

	// gui main loop

	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		log.Panicln(err)
	}

	g.Close()

	//fmt.Printf("VBUF:\n%s\n", vbuf)
	//fmt.Printf("BUF:\n%s\n", buf)
}

func Worker() {
	data := &protobuf.Header{
		Uuid: name,
		Contain: &protobuf.Header_SetNameReq{
			SetNameReq: &protobuf.SetNameReq{
				Username: name,
			},
		},
	}
	dataBuffer, _ := proto.Marshal(data)
	err := conn.WriteMessage(websocket.TextMessage, dataBuffer)

	if err != nil {
		log.Println(err)
		return
	}

	req := MessageReq("Hello Everyone\n")
	conn.WriteMessage(websocket.TextMessage, req)

	go heartBeat()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			// break
		}

		var head pb.Header
		proto.Unmarshal(msg, &head)
		//fmt.Println("[Response] Code =", head.Code)
		switch head.Contain.(type) {
		case *pb.Header_MessageNotify:
			contain := head.GetMessageNotify()

			guiInstance.Update(func(g *gocui.Gui) error {
				fmt.Fprint(show, " ", contain.Username, ":", contain.Message)
				return nil
			})
		case *pb.Header_HeartBeat:
			//fmt.Println("Header_HeartBeat!!", result)
		case nil:
			//fmt.Println("response of", head.GetUuid())
		default:
			//fmt.Printf("nope")
		}

		if err != nil {
			log.Println("write:", err)
			break
		}
	}
}

func heartBeat() {
	for {
		data := &protobuf.Header{
			Uuid: name,
			Contain: &protobuf.Header_HeartBeat{
				HeartBeat: &protobuf.HeartBeat{},
			},
		}
		dataBuffer, _ := proto.Marshal(data)
		conn.WriteMessage(websocket.TextMessage, dataBuffer)
		time.Sleep(5 * time.Second)
	}
}

func loadCA(caFile string) *x509.CertPool {
	pool := x509.NewCertPool()
	if ca, e := ioutil.ReadFile(caFile); e != nil {
		log.Fatal("ReadFile: ", e)
	} else {
		pool.AppendCertsFromPEM(ca)
	}
	return pool
}

// MessageReq :
func MessageReq(message string) []byte {
	//fmt.Println("[MessageReq] message = ", message)
	data := &pb.Header{
		Uuid: name,
		Contain: &pb.Header_MessageReq{
			MessageReq: &pb.MessageReq{
				Message: message,
			},
		},
	}

	dataBuffer, _ := proto.Marshal(data)
	return dataBuffer
}
