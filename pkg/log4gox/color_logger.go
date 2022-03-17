package log4gox

import (
	"fmt"
	"io"
	"os"

	l4g "github.com/alecthomas/log4go"
)

var stdout io.Writer = os.Stdout

/*
前景色            背景色           颜色
---------------------------------------
30                40              黑色
31                41              红色
32                42              绿色
33                43              黃色
34                44              蓝色
35                45              紫红色
36                46              青蓝色
37                47              白色
*/
var (
	levelColor   = [...]int{30, 30, 32, 37, 37, 33, 31, 34}
	levelStrings = [...]string{"FNST", "FINE", "DEBG", "TRAC", "INFO", "WARN", "EROR", "CRIT"}
)

/*
fmt.Printf("%c[%dm(1 2 3 4)%c[0m ", 0x1B, 30, 0x1B)

for b := 40; b <= 47; b++ { // 背景色彩 = 40-47
	for f := 30; f <= 37; f++ { // 前景色彩 = 30-37
		for d := range []int{0, 1, 4, 5, 7, 8} { // 显示方式 = 0,1,4,5,7,8
			fmt.Fprintf(os.Stderr, " %c[%d;%d;%dm%s(1 2 3 4 )%c[0m ", 0x1B, d, b, f, "", 0x1B)
		}
		fmt.Println("")
	}
	fmt.Println("")
}
*/

const (
	colorSymbol = 0x1B
)

// This is the standard writer that prints to standard output.
type ConsoleLogWriter chan *l4g.LogRecord

// This creates a new ConsoleLogWriter
func NewColorConsoleLogWriter() ConsoleLogWriter {
	records := make(ConsoleLogWriter, l4g.LogBufferLength)
	go records.run(stdout)
	return records
}

func (w ConsoleLogWriter) run(out io.Writer) {
	var timestr string
	var timestrAt int64

	for rec := range w {
		if at := rec.Created.UnixNano() / 1e9; at != timestrAt {
			timestr, timestrAt = rec.Created.Format("01/02/06 15:04:05"), at
		}
		fmt.Fprintf(out, "%c[%dm[%s] [%s] (%s) %s\n%c[0m",
			colorSymbol,
			levelColor[rec.Level],
			timestr,
			levelStrings[rec.Level],
			rec.Source,
			rec.Message,
			colorSymbol)
	}
}

// This is the ConsoleLogWriter's output method.  This will block if the output
// buffer is full.
func (w ConsoleLogWriter) LogWrite(rec *l4g.LogRecord) {
	w <- rec
}

// Close stops the logger from sending messages to standard output.  Attempts to
// send log messages to this logger after a Close have undefined behavior.
func (w ConsoleLogWriter) Close() {
	close(w)
}
