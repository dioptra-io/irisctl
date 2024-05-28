package analyze

import (
	"fmt"
	"image/color"
	"strconv"
	"time"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
)

func dotChart(measPerHour map[string]map[string]int) error {
	p := plot.New()
	p.Title.Text = "Number of Measurements by Hours"
	p.X.Label.Text = "Hour of Day"
	p.Y.Label.Text = "Date"
	xy, values := initXY(measPerHour)
	p.X.Tick.Marker = hourTicks{xy}
	p.Y.Tick.Marker = dateTicks{xy}
	labels, err := plotter.NewLabels(plotter.XYLabels{XYs: xy, Labels: values})
	if err != nil {
		return err
	}
	p.Add(labels)

	s, err := plotter.NewScatter(xy)
	if err != nil {
		return err
	}
	s.GlyphStyleFunc = func(i int) draw.GlyphStyle {
		c := color.RGBA{R: 196, B: 128, A: 255}
		if values[i] == "" || values[i] == "0" {
			return draw.GlyphStyle{Color: c, Radius: vg.Length(0), Shape: nil}
		}
		r, err := strconv.ParseFloat(values[i], 32)
		if err != nil {
			panic(err)
		}
		r = (r * 65) / 260
		if r < 1 {
			r = 1
		}
		return draw.GlyphStyle{Color: c, Radius: vg.Length(r), Shape: draw.CircleGlyph{}}
	}
	p.Add(s)
	return p.Save(12*vg.Inch, 240*vg.Inch, "measurements_per_hour.svg")
}

func initXY(measPerHour map[string]map[string]int) (plotter.XYs, []string) {
	var xy plotter.XYs
	var labels []string
	for date := range measPerHour {
		d, err := time.Parse("2006-01-02", date)
		if err != nil {
			panic(err)
		}
		if _, ok := measPerHour[date]; !ok {
			fmt.Printf("INTERNAL ERROR: date=%v\n", date)
			panic("corrupted measPerHour map")
		}
		for h := 0; h < 24; h++ {
			hour := fmt.Sprintf("%02d", h)
			n, ok := measPerHour[date][hour]
			if !ok {
				fmt.Printf("INTERNAL ERROR: date=%v hour=%v\n", date, hour)
				panic("corrupted measPerHour map")
			}
			skip := true
			for _, hh := range hours {
				if measPerHour[date][hh] != 0 {
					skip = false
					break
				}
			}
			if skip {
				continue
			}
			xy = append(xy, struct{ X, Y float64 }{float64(h), float64(d.Unix())})
			if n == 0 {
				labels = append(labels, "")
			} else {
				labels = append(labels, fmt.Sprintf("%d", n))
			}
		}
	}
	return xy, labels
}

// hourTicks implements the Ticker interface for hours.
type hourTicks struct {
	xy plotter.XYs
}

// Ticks returns the tick positions and labels.
func (t hourTicks) Ticks(min, max float64) []plot.Tick {
	var ticks []plot.Tick
	for hour := 0; hour < 24; hour++ {
		ticks = append(ticks, plot.Tick{Value: float64(hour), Label: fmt.Sprintf("%02d", hour)})
	}
	return ticks
}

// dateTicks implements the Ticker interface for dates.
type dateTicks struct {
	xy plotter.XYs
}

// Ticks returns the tick positions and labels.
func (t dateTicks) Ticks(min, max float64) []plot.Tick {
	var ticks []plot.Tick
	uniqueDates := make(map[time.Time]bool)
	for _, point := range t.xy {
		uniqueDates[time.Unix(int64(point.Y), 0).Truncate(24*time.Hour)] = true
	}
	for date := range uniqueDates {
		ticks = append(ticks, plot.Tick{Value: float64(date.Unix()), Label: date.Format("2006-01-02")})
	}
	return ticks
}
