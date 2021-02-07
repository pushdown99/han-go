package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/getlantern/systray"
	"github.com/getlantern/systray/example/icon"
	"github.com/gorilla/websocket"
	"github.com/jacobsa/go-serial/serial"
	"github.com/joho/godotenv"
	"github.com/webview/webview"
)

var httpHost      string
var wsHost        string
var rcnNum        string
var macAddr       string
var ipAddr        string
var license       string
var postReceipt   string
var postLicense   string
var wsReceipt     string
var comPrinter    string
var comCom0Com    string
var wsConnected   int = 0
var httpConnected int = 0

func GetOutboundIP() net.IP {
   conn, err := net.Dial("udp", "8.8.8.8:80")
   if err != nil { log.Fatal(err) }
   defer conn.Close()
   return conn.LocalAddr().(*net.UDPAddr).IP
}

func GetOutboundMac(currentIP string) string {
   var currentNetworkHardwareName string
   interfaces, _ := net.Interfaces()
   for _, interf := range interfaces {
      if addrs, err := interf.Addrs(); err == nil {
         for _, addr := range addrs {
            if strings.Contains(addr.String(), currentIP) { currentNetworkHardwareName = interf.Name }
         }
      }
   }
   netInterface, err := net.InterfaceByName(currentNetworkHardwareName)
   if err != nil { fmt.Println(err) }
   return netInterface.HardwareAddr.String()
}

type License struct {
   Mac       string
   Rcn       string
}

func getConfig () bool {
   err := godotenv.Load()
   if err != nil { log.Fatal("Error loading .env file") }
   httpHost = os.Getenv("SERVER")
   wsHost   = os.Getenv("WS")
   rcnNum   = os.Getenv("RCN")
   comPrinter = os.Getenv("PRINTER")
   comCom0Com = os.Getenv("COM0COM")
   ipAddr   = GetOutboundIP().String()
   macAddr  = GetOutboundMac(ipAddr)

   //b, _ := json.Marshal(License {mac: macAddr, Timestamp: time.Now().Unix()})
   d := License {Mac: macAddr, Rcn: rcnNum}
   b, _ := json.Marshal(d)
   resp, err := http.Post(httpHost + "/pos/sign-in/", "application/json", bytes.NewBuffer(b))
   if err != nil { 
      log.Printf("Error: ", err)
   } 
   if resp.StatusCode == http.StatusOK {
      s, err := ioutil.ReadAll(resp.Body)
      if err == nil {
         var result map[string]interface{}
         json.Unmarshal([]byte(s), &result)
         if(result["code"].(float64) != 200) {
            myBrowser(httpHost + "/pos/sign-up/" + macAddr)
            return false
         }
         license = result["license"].(string)
         log.Printf("License: %s", license)
         return true
      }
   }
   return false
}

////////////////////////////////////////////////////////////////////////////////////////////////////

type JsonData struct {
   Data      string
   Timestamp int64
}

func Open(device string) io.ReadWriteCloser {
   options := serial.OpenOptions{
      PortName:        device,
      BaudRate:        19200,
      DataBits:        8,
      StopBits:        1,
      MinimumReadSize: 4,
   }
   port, err := serial.Open(options)
   if err != nil { log.Printf("serial.Open: %v", err) }
   return port
}

func Run(in io.ReadWriteCloser, out io.ReadWriteCloser, wg *sync.WaitGroup) {
   defer wg.Done()

   for {
      buf := make([]byte, 4096)
      n, err := in.Read(buf)

      if err != nil {
         if err != io.EOF { log.Fatal("[err] reading from serial port: ", err) }
      } else {
         buf = buf[:n]
         if n > 0 {
            b, _ := json.Marshal(JsonData{Data: hex.EncodeToString(buf), Timestamp: time.Now().Unix()})
            //resp, err := http.Post(httpHost + "/receipt/probe/" + license, "application/json", bytes.NewBuffer(b))
            resp, err := http.Post(httpHost + "/receipt/probe/1234", "application/json", bytes.NewBuffer(b))
            if err != nil {
               log.Printf("Error: ", err)
            } else {
               defer resp.Body.Close()
               if resp.StatusCode == http.StatusOK {
                  s, err := ioutil.ReadAll(resp.Body)
                  if err == nil {
                     b, _ = hex.DecodeString(string(s))
                     fmt.Println(b)
                     out.Write(b)
                  }
               }
            }
         }
         //out.Write(buf)
      }
   }
}

////////////////////////////////////////////////////////////////////////////////////////////////////
var title = "hancom agent"

func onReady() {
   systray.SetTemplateIcon(icon.Data, icon.Data)
   systray.SetTitle(title)
   systray.SetTooltip(title)
   mQRcode  := systray.AddMenuItem("QR code", "QR code reader pairing")
   mSetting := systray.AddMenuItem("Setting", "Setting agent")
//   mQuit    := systray.AddMenuItem("Quit", "Quit agent")
//   go func() {
//      <-mQuit.ClickedCh
//      systray.Quit()
//   }()

   for {
      select {
      case <-mSetting.ClickedCh:
         myBrowser(httpHost + "/pos/registered/" + license)
      case <-mQRcode.ClickedCh:
         myBrowser(httpHost + "/pos/pairing/" + license)
//      case <-mQuit.ClickedCh:
//         systray.Quit()
//         return
      }
   }
}

func mySystray(wg *sync.WaitGroup) {
   defer wg.Done()

   onExit := func() {}
   systray.Run(onReady, onExit)
}

type WsData struct {
   License   string
   Command   string
   Message   string
   Timestamp int64
}

func connectWS () *websocket.Conn {
   for {
      u := url.URL{Scheme: "ws", Host: wsHost, Path: "/"}
      c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
      if err != nil {
         log.Printf("dial:", err)
         time.Sleep (1)
         continue;
      }
      log.Printf("[log] connected to %s", u.String())
      wsConnected = 1
      b, _ := json.Marshal(WsData{License: license, Command: "Join", Message: "Hi", Timestamp: time.Now().Unix()})
      c.WriteMessage (websocket.TextMessage, b/*[]byte(b)*/)
      return c
   }
}

func myBrowser(url string) {
	debug := true
	w := webview.New(debug)
	defer w.Destroy()
	w.SetTitle("Minimal webview example")
	w.SetSize(800, 600, webview.HintMax)
	w.Navigate(url)
	w.Run()
}

func myWS(wg *sync.WaitGroup) {
   defer wg.Done()

   c := connectWS ()
   defer c.Close()

   done := make(chan struct{})

   go func() {
      defer close(done)
      for {
         if(wsConnected == 0) {
            time.Sleep (1)
            c = connectWS ()
         }
         _, message, err := c.ReadMessage()
         if err != nil {
            log.Println("read:", err)
            wsConnected = 0
            continue
         }
         log.Printf("[log] received message: %s", message)
         
         var result map[string]interface{}
         json.Unmarshal([]byte(message), &result)
         if(result["Command"].(string) == "Callback") {
            //log.Printf("callback %s", result["Message"].(string))
            go myBrowser (result["Message"].(string))
         }
         log.Printf(string(message))         
      }
   }()
   ticker := time.NewTicker(time.Second)
   defer ticker.Stop()

   for {
      select {
      case <-done:
         return
      }
   }
}


////////////////////////////////////////////////////////////////////////////////////////////////////
var logo = ` 
========================== 
Hancom smart receipt agent 
========================== 
`
func main() {

   log.Printf(logo)
   if (getConfig() == false) {
      log.Printf("Please, check your POS configuration.");
      return
   }

   var wg sync.WaitGroup
   wg.Add(1)
   go myWS(&wg)

   wg.Add(1)
   go mySystray(&wg)

   in := Open(comCom0Com)
   out := Open(comPrinter)

   wg.Add(2)
   go Run(in, out, &wg)
   go Run(out, in, &wg)

   wg.Wait()
}