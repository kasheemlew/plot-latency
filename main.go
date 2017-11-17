package main

import (
	"fmt"
	"image/color"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/pkg/errors"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg/draw"
)

type server struct {
	data []time.Duration
	sync.RWMutex
}

func main() {
	var s server
	addr := "localhost:8080"

	rand.Seed(time.Now().Unix())
	http.HandleFunc("/", s.root)
	http.HandleFunc("/statz", s.statz)
	http.HandleFunc("/statz/scatter.png", errorHandler(s.scatter))
	log.Println("listening on", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func (s *server) root(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	x := 1000 * rand.Float64()
	t := time.Duration(x) * time.Millisecond
	fmt.Fprintln(w, "slept for", t)

	s.Lock()
	s.data = append(s.data, t)
	if len(s.data) > 1000 {
		s.data = s.data[len(s.data)-1000:]
	}
	s.Unlock()
}

func (s *server) statz(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "%s", `
                <h1>Latency Stats</h1>
                <img src="/statz/scatter.png?rand=0" style="width:50%">
                <script>
                setInterval(function() {
                        var imgs = document.getElementsByTagName("IMG");
                        for (var i=0; i<imgs.length; i++) {
                                var p = imgs[i].src.lastIndexOf("=");
                                imgs[i].src = imgs[i].src.substr(0, p+1) + Math.random();
                        }
                }, 100)
                </script>
                `)
}

func (s *server) scatter(w http.ResponseWriter, r *http.Request) error {
	s.RLock()
	defer s.RUnlock()

	xys := make(plotter.XYs, len(s.data))
	for i, d := range s.data {
		xys[i].X = float64(i)
		xys[i].Y = float64(d) / float64(time.Millisecond)
	}

	sc, err := plotter.NewScatter(xys)
	if err != nil {
		return errors.Wrap(err, "could not create newscatter")
	}
	sc.GlyphStyle.Shape = draw.CrossGlyph{}

	avgs := make(plotter.XYs, len(s.data))
	sum := 0.0
	for i, d := range s.data {
		avgs[i].X = float64(i)
		sum += float64(d)
		avgs[i].Y = sum / (float64(i+1) * float64(time.Millisecond))
	}
	l, err := plotter.NewLine(avgs)
	if err != nil {
		return errors.Wrap(err, "could not plotter newline")
	}
	l.Color = color.RGBA{G: 255, A: 255}

	g := plotter.NewGrid()
	g.Horizontal.Color = color.RGBA{R: 255, A: 255}
	g.Vertical.Width = 0

	p, err := plot.New()
	if err != nil {
		return errors.Wrap(err, "could not plot new")
	}
	p.Add(sc, l, g)
	p.Title.Text = "Endpoint Latency"
	p.X.Label.Text = "sample"
	p.Y.Label.Text = "ms"

	wt, err := p.WriterTo(512, 512, "png")
	if err != nil {
		return errors.Wrap(err, "could not write to")
	}

	_, err = wt.WriteTo(w)
	return errors.Wrap(err, "could not write to")
}

func errorHandler(h func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := h(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}
