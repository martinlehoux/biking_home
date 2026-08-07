package chart

import (
	"fmt"
	"image/color"

	"github.com/martinlehoux/biking_home/mountain_pass"
	"github.com/martinlehoux/biking_home/ride"
	"github.com/martinlehoux/kagamigo/kcore"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
)

func Render(r ride.Ride, climbs []ride.Climb, crossings []mountain_pass.Crossing, output string) {
	p := plot.New()
	p.Title.Text = "Ride profile"
	p.X.Label.Text = "Distance (km)"
	p.Y.Label.Text = "Elevation (m)"

	maxElev := 0.0
	elevation := make(plotter.XYs, r.Len())
	for i := 0; i < r.Len(); i++ {
		elevation[i].X = r.DistanceM(i) / 1000
		elevation[i].Y = r.ElevationM(i)
		if r.ElevationM(i) > maxElev {
			maxElev = r.ElevationM(i)
		}
	}

	for _, climb := range climbs {
		x0 := climb.StartDistanceM() / 1000
		x1 := climb.EndDistanceM() / 1000
		fill, err := plotter.NewPolygon(plotter.XYs{
			{X: x0, Y: 0},
			{X: x1, Y: 0},
			{X: x1, Y: maxElev},
			{X: x0, Y: maxElev},
		})
		kcore.Expect(err, "failed to create climb band")
		fill.Color = color.RGBA{R: 230, G: 160, B: 0, A: 70}
		fill.LineStyle.Width = 0
		p.Add(fill)
		p.Add(plotLabels(plotter.XY{X: (x0 + x1) / 2, Y: climb.TopElevationM() * 1.01}, climbLabel(climb)))
	}

	for _, crossing := range crossings {
		x := crossing.RideDistanceM / 1000
		line, err := plotter.NewLine(plotter.XYs{
			{X: x, Y: 0},
			{X: x, Y: maxElev},
		})
		kcore.Expect(err, "failed to create pass marker")
		line.LineStyle.Dashes = []vg.Length{vg.Points(4), vg.Points(2)}
		p.Add(line)
		p.Add(plotLabels(plotter.XY{X: x, Y: maxElev * 0.96},
			fmt.Sprintf("%s %dm", crossing.Pass.Name, crossing.Pass.Elevation)))
	}

	elevLine, err := plotter.NewLine(elevation)
	kcore.Expect(err, "failed to create elevation line")
	p.Add(elevLine)

	kcore.Expect(p.Save(10*vg.Inch, 4*vg.Inch, output), "failed to save chart")
}

func climbLabel(climb ride.Climb) string {
	if climb.Name != "" {
		return climb.Name
	}
	return fmt.Sprintf("%s %d", ride.Category(climb.Score()), int(climb.Score()))
}

func plotLabels(xy plotter.XY, label string) *plotter.Labels {
	labels, err := plotter.NewLabels(plotter.XYLabels{
		XYs:    plotter.XYs{xy},
		Labels: []string{label},
	})
	kcore.Expect(err, "failed to create labels")
	return labels
}
