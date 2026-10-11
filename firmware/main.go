// wio-gopher: Wio Terminal の 40pin ヘッダに載せた WS2812B を光らせる。
//
// 回路図 (wio-gopher.kicad_sch) の接続:
//
//	J1 pin 7  (GCLK0 / PB14 / BCM4) -> D1 DIN
//	D1 DOUT                         -> D2 DIN  (D2 DOUT は未接続)
//	J1 pin 1  (3V3)                 -> D1, D2 VDD
//	J1 GND                          -> D1, D2 VSS
//
// 点灯パターンはパチンコ台風に、次の流れを繰り返す:
//
//	通常 (高速レインボー) -> リーチ (赤青交互が加速) -> 抽選
//	  -> 大当たり (ストロボ連発) / ハズレ (赤がフェードアウト)
//
// 書き込み: tinygo flash -target=wioterminal .
package main

import (
	"image/color"
	"machine"
	"time"

	"tinygo.org/x/drivers/ws2812"
)

const (
	ledPin      = machine.BCM4 // = PB14
	numLEDs     = 2
	brightness  = 255 // 3.3V 給電で不安定になるなら下げる
	jackpotOdds = 3   // 1/jackpotOdds で大当たり
)

var (
	black = color.RGBA{}
	white = color.RGBA{R: 255, G: 255, B: 255}
	red   = color.RGBA{R: 255}
	blue  = color.RGBA{B: 255}
	gold  = color.RGBA{R: 255, G: 160}

	// 大当たり時に順番にフラッシュする 7 色
	sevenColors = []color.RGBA{
		{R: 255},
		{R: 255, G: 80},
		{R: 255, G: 200},
		{G: 255},
		{G: 200, B: 255},
		{B: 255},
		{R: 180, B: 255},
	}
)

var (
	leds ws2812.Device
	buf  [numLEDs]color.RGBA
)

func main() {
	// 40pin ヘッダの 3V3 出力を有効にする (PC15, Low で ON)。
	// TinyGo は起動時にこのピンを設定しないため、明示しないと LED に電源が来ない。
	machine.OUTPUT_CTR_3V3.Configure(machine.PinConfig{Mode: machine.PinOutput})
	machine.OUTPUT_CTR_3V3.Low()
	time.Sleep(10 * time.Millisecond)

	ledPin.Configure(machine.PinConfig{Mode: machine.PinOutput})
	leds = ws2812.NewWS2812(ledPin)
	leds.SetBrightness(brightness)

	for {
		normal(3*time.Second, 6)
		reach()
		if lottery() {
			jackpot()
		} else {
			miss()
		}
	}
}

// normal は通常時の演出。2 個の LED を逆位相で虹色回転させる。
// speed は 10ms ごとに進める色相の量。
func normal(d time.Duration, speed uint8) {
	const step = 10 * time.Millisecond
	var hue uint8
	for t := time.Duration(0); t < d; t += step {
		show(wheel(hue), wheel(hue+128))
		hue += speed
		time.Sleep(step)
	}
}

// reach はリーチ演出。赤と青を左右交互に点滅させ、だんだん加速する。
func reach() {
	for interval := 200 * time.Millisecond; interval > 25*time.Millisecond; interval = interval * 85 / 100 {
		show(red, blue)
		time.Sleep(interval)
		show(blue, red)
		time.Sleep(interval)
	}

	// 溜め: 一瞬消灯して抽選結果を待たせる
	fill(black)
	time.Sleep(600 * time.Millisecond)
}

// jackpot は大当たり演出。白ストロボ -> 7 色フラッシュ -> 金赤交互 -> 超高速レインボー。
func jackpot() {
	strobe(white, 30, 15*time.Millisecond, 35*time.Millisecond)

	for range 3 {
		for _, c := range sevenColors {
			strobe(c, 2, 25*time.Millisecond, 25*time.Millisecond)
		}
	}

	for range 20 {
		show(gold, red)
		time.Sleep(60 * time.Millisecond)
		show(red, gold)
		time.Sleep(60 * time.Millisecond)
	}

	normal(2*time.Second, 24)
	strobe(white, 10, 15*time.Millisecond, 35*time.Millisecond)
}

// miss はハズレ演出。赤をゆっくりフェードアウトさせる。
func miss() {
	for v := 255; v >= 0; v -= 5 {
		fill(color.RGBA{R: uint8(v)})
		time.Sleep(15 * time.Millisecond)
	}
	time.Sleep(500 * time.Millisecond)
}

// strobe は全 LED を色 c で n 回点滅させる。
func strobe(c color.RGBA, n int, on, off time.Duration) {
	for range n {
		fill(c)
		time.Sleep(on)
		fill(black)
		time.Sleep(off)
	}
}

// lottery は大当たり抽選。SAMD51 のハードウェア乱数 (TRNG) を使う。
func lottery() bool {
	r, err := machine.GetRNG()
	if err != nil {
		return false
	}
	return r%jackpotOdds == 0
}

// show は D1 と D2 に個別の色を出す。
func show(d1, d2 color.RGBA) {
	buf[0], buf[1] = d1, d2
	leds.WriteColors(buf[:])
}

// fill は全 LED を同じ色にする。
func fill(c color.RGBA) {
	for i := range buf {
		buf[i] = c
	}
	leds.WriteColors(buf[:])
}

// wheel は色相 (0-255) を R→G→B→R と一周する色に変換する。
func wheel(pos uint8) color.RGBA {
	switch {
	case pos < 85:
		return color.RGBA{R: 255 - pos*3, G: pos * 3, B: 0}
	case pos < 170:
		pos -= 85
		return color.RGBA{R: 0, G: 255 - pos*3, B: pos * 3}
	default:
		pos -= 170
		return color.RGBA{R: pos * 3, G: 0, B: 255 - pos*3}
	}
}
