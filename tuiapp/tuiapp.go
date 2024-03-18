package tuiapp

import (
	//"context"
	"io"
	"time"

	"container/ring"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	//"go.einride.tech/can/pkg/socketcan"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tickMsg time.Time
type sessionState uint

const (
	padding                = 2
	timerView sessionState = iota
	spinnerView
	maxWidth  = 80
	MAX_SPEED = 0x0fff
)

var (
	debug_s   string
	w, h      int
	motor_id  [10]int
	can_pkg   [8]uint8
	can_r     *ring.Ring
	io_reader io.Reader
	dump_cmd  *exec.Cmd
)

type model struct {
	state    sessionState
	spinner  spinner.Model
	progress progress.Model
}

// NOTE: Styles
var (
	appStyle = lipgloss.NewStyle().Padding(1, 2)
	spinners = []spinner.Spinner{
		spinner.Line,
		spinner.Dot,
		spinner.MiniDot,
		spinner.Jump,
		spinner.Pulse,
		spinner.Points,
		spinner.Globe,
		spinner.Moon,
		spinner.Monkey,
	}

	spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))
	helpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).PaddingLeft(3)

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#fbf1c7")).
			Background(lipgloss.Color("#bd93f9")).
			Padding(0, 1)

	onlineStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#458588")).
			Background(lipgloss.Color("#50fa7b")).
			Padding(0, 1)

	offlineStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#458588")).
			Background(lipgloss.Color("#ff5555")).
			Padding(0, 1)

	modelStyle = lipgloss.NewStyle().
			Width(110).
			Height(50).
			Align(lipgloss.Center, lipgloss.Center).
			BorderStyle(lipgloss.HiddenBorder())
	focusedModelStyle = lipgloss.NewStyle().
				Width(110).
				Height(50).
				Align(lipgloss.Center, lipgloss.Center).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("69"))
)

func New() *tea.Program {
	m := model{
		progress: progress.New(progress.WithDefaultGradient()),
		spinner:  spinner.New(),
	}

	m.spinner.Style = spinnerStyle
	m.spinner.Spinner = spinners[4]
	m.progress.Width = 80

	p := tea.NewProgram(m)

	return p
}

func (m model) Init() tea.Cmd {
	var err error
	dump_cmd := exec.Command("candump", "can1")
	io_reader, err = dump_cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}
	if err := dump_cmd.Start(); err != nil {
		panic(err)
	}

	// Error handling omitted to keep example simple
	//conn, _ := socketcan.DialContext(context.Background(), "can", "can0")

	//recv := socketcan.NewReceiver(conn)
	//for recv.Receive() {
	//    frame := recv.Frame()
	//    fmt.Println(frame.String())
	//}

	can_r = ring.New(5)
	return tea.Batch(tea.EnterAltScreen, tickCmd(), m.spinner.Tick)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case tea.KeyPgUp.String():
			cmd := m.progress.IncrPercent(0.05)
			return m, tea.Batch(cmd)

		case tea.KeyPgDown.String():
			cmd := m.progress.DecrPercent(0.05)
			return m, tea.Batch(cmd)

		case "tab":
			if m.state == timerView {
				m.state = spinnerView
			} else {
				m.state = timerView
			}
		case "q":
			return m, tea.Quit

		default:
			return m, nil

		}

	case tea.WindowSizeMsg:
		w = msg.Width
		h = msg.Height
		modelStyle.Width(w/2 - 2)
		modelStyle.Height(h - 6)
		focusedModelStyle.Height(h - 6)
		focusedModelStyle.Width(w/2 - 2)

		return m, nil

	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		return m, tea.Batch(tickCmd(), cansend(5, (int)(MAX_SPEED*m.progress.Percent())), cmd)

	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd

	default:
		return m, nil
	}
	return m, nil
}

func (m model) View() string {
	var s, can_buf string
	motor_s := titleStyle.Render("Motor status") + "\n\n"
	pad := strings.Repeat(" ", padding)
	pad2 := strings.Repeat(" ", padding*34+1)
	var IDs []string
	for _, i := range motor_id {
		IDs = append(IDs, strconv.Itoa(i))
	}
	debug := fmt.Sprintf("%f", m.progress.Percent()) +
		pad + debug_s + "min" + pad2 + "max\n\n"

	buf := make([]byte, 100)
	n, err := io_reader.Read(buf)
	if err != nil {
		panic(err)
	}

	for i := 0; i < n; i++ {
		if buf[i] == 'c' && i+42 < n {
			can_r.Value = fmt.Sprintln(string(buf[i : i+42]))
			can_r = can_r.Next()
		}
	}

	for i := 0; i < 4; i++ {
		if strings.Contains(fmt.Sprintln(string(buf[0:n])), fmt.Sprintf("20%d", i+1)) {
			motor_s += fmt.Sprintf("Motor %d : %s\n", i, onlineStyle.Render(" ONLINE👌"))
		} else {
			motor_s += fmt.Sprintf("Motor %d : %s\n", i, offlineStyle.Render("OFFLINE🚫"))
		}
	}

	can_buf += "\n\n"
	can_r.Do(func(s any) {
		can_buf += fmt.Sprintln(s)
	})
	can_buf += "\n"

	if m.state == timerView {
		s += lipgloss.JoinHorizontal(lipgloss.Top, focusedModelStyle.Render(debug, m.progress.View()),
			modelStyle.Render(motor_s, can_buf, m.spinner.View(),
				m.spinner.View(), m.spinner.View(), m.spinner.View(), m.spinner.View(), m.spinner.View()))
	} else {
		s += lipgloss.JoinHorizontal(lipgloss.Top, modelStyle.Render(m.progress.View()),
			focusedModelStyle.Render(motor_s, can_buf, m.spinner.View(),
				m.spinner.View(), m.spinner.View(), m.spinner.View(), m.spinner.View(), m.spinner.View()))
	}
	s += helpStyle.Render("\ntab: focus next • q: exit\n")

	return s
}

func cansend(num int, speed int) tea.Cmd {
	debug_s = fmt.Sprintf("set speed : %d\n", speed)

	for i := 0; i < 4; i++ {
			can_pkg[2*i] = (uint8)(speed >> 8)
			can_pkg[2*i+1] = (uint8)(speed)
	}

	debug_s += fmt.Sprintf("  can_pkg : %02x %02x %02x %02x %02x %02x %02x %02x\n\n",
		can_pkg[0], can_pkg[1], can_pkg[2], can_pkg[3],
		can_pkg[4], can_pkg[5], can_pkg[6], can_pkg[7])
	can_s := fmt.Sprintf("%02x%02x%02x%02x%02x%02x%02x%02x",
		can_pkg[0], can_pkg[1], can_pkg[2], can_pkg[3],
		can_pkg[4], can_pkg[5], can_pkg[6], can_pkg[7])

	cmd := exec.Command("cansend", "can1", "200#"+can_s)
	_, err := cmd.Output()
	if err != nil {
		fmt.Println(err.Error())
	}

	return nil
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Microsecond*1, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
