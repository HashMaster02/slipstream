package generate

import (
	"math"
	"time"
)

type Candle struct {
	Timestamp time.Time
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    int64
}

type Quote struct {
	Timestamp time.Time
	Bid  	  float64
	Ask  	  float64
	Last 	  float64
}

func HighLowAsAskBid(dayT Candle) Quote {
	/*
		Considers the High as the Ask, Low as the Bid, and Close as the Last
	*/
	var ask = dayT.High
	var bid = dayT.Low
	return Quote{Timestamp: dayT.Timestamp, Bid: bid, Ask: ask, Last: dayT.Close}
}

func CorwinSchultz(dayT1 Candle, dayT2 Candle) Quote {
	/*
		This function uses the Corwin-Schultz Estimator to estimate
		Bid and Ask values from High and Low values of 2 consecutive
		candles. The estimator makes some assumptions:

		- Stocks trade continuously
		- Stock values don't change while the market is closed

		In our implementation, the Close of dayT2 is the Last. 
		The estimator was tested on the Daily timeframe by the original researchers.
	*/

	var H_t_t1 float64 = math.Max(dayT1.High, dayT2.High)
	var L_t_t1 float64 = math.Min(dayT1.Low, dayT2.Low)

	var beta float64 = math.Pow(math.Log(dayT1.High / dayT1.Low), 2) + math.Pow(math.Log(dayT2.High / dayT2.Low), 2)

	var gamma float64 = math.Pow(math.Log(H_t_t1 / L_t_t1), 2)

	var alpha float64 = (math.Sqrt(2) + 1) * (math.Sqrt(beta) - math.Sqrt(gamma))

	var S float64 = (2 * (math.Exp(alpha) - 1)) / (1 + math.Exp(alpha))
	if S < 0 {
		S = 0
	}

	var ask = dayT2.Close * (1 + (S/2))
	var bid = dayT2.Close * (1 - (S/2))

	return Quote{Timestamp: dayT2.Timestamp, Bid: bid, Ask: ask, Last: dayT2.Close}

}