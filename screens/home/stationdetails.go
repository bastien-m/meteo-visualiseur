package home

import (
	"database/sql"
	"fmt"
	"log/slog"
	"meteo/common"
	"meteo/components/ui"
	appcontext "meteo/context"
	"meteo/data"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/s-daehling/fyne-charts/pkg/coord"
	chartData "github.com/s-daehling/fyne-charts/pkg/data"
)

type StationDetailsComponent struct {
	w           fyne.Window
	db          *sql.DB
	logger      *slog.Logger
	camera      common.Position
	dimension   common.Dimension
	needRefresh binding.Bool
	graphMode   binding.Int
}

func InitStationDetailsComponent(dimension common.Dimension) *StationDetailsComponent {
	graphModeBinding := binding.NewInt()
	graphModeBinding.Set(int(ui.NORMAL))
	appContext := appcontext.GetAppContext()
	return &StationDetailsComponent{
		w:         appContext.W,
		logger:    appContext.Logger,
		db:        appContext.DB,
		dimension: dimension,
		camera: common.Position{
			X: 0,
			Y: 0,
			Z: 1,
		},
		needRefresh: binding.NewBool(),
		graphMode:   graphModeBinding,
	}
}

func (c *StationDetailsComponent) Render(station *data.StationInfo) *fyne.Container {
	chart := coord.NewCartesianTemporalChart("Rain")
	chart.SetTAxisLabel("Year")
	chart.SetYAxisLabel("mm")

	rainByYear, err := data.GetRainByStation(c.db, station.NumPost)

	if err != nil {
		c.logger.Error(fmt.Sprint("Error while fetching data %w", err))
		return nil
	}

	fmt.Println(len(rainByYear))

	// serieData := make([]chartData.TemporalPoint, len(rainByYear))

	// for i, record := range rainByYear {
	// 	y, err := strconv.Atoi(record.Year)
	// 	if err != nil {
	// 		continue
	// 	}
	// 	serieData[i] = chartData.TemporalPoint{
	// 		T:   time.Date(y, 0, 0, 0, 0, 0, 0, time.Local),
	// 		Val: record.Rain,
	// 	}
	// }

	tps, err := coord.NewTemporalPointSeries("rain", theme.ColorNamePrimary, []chartData.TemporalPoint{{
		T:   time.Now().Add(time.Hour * 10),
		Val: 1,
	}, {
		T:   time.Now().Add(time.Hour * 9),
		Val: 2,
	}, {
		T:   time.Now().Add(time.Hour * 8),
		Val: 3,
	}})
	if err != nil {
		return nil
	}

	err = chart.AddLineSeries(tps, true)
	if err != nil {
		return nil
	}

	dataContainer := container.NewGridWithColumns(2)

	for _, record := range rainByYear {
		dataContainer.Add(widget.NewLabel(record.Year))
		dataContainer.Add(widget.NewLabel(fmt.Sprintf("%.1f", record.Rain)))
	}

	vbox := container.NewVBox(
		// chart,
		dataContainer,
	)

	return vbox
}
