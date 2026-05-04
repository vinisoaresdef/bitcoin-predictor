(function(global) {
  'use strict';

  const chartContainer = document.getElementById('chart-container');
  if (!chartContainer) {
    console.error('Chart container not found: #chart-container');
    return;
  }

  const chartOptions = {
    layout: {
      background: { color: '#1a1a2e' },
      textColor: '#e0e0e0',
    },
    grid: {
      vertLines: { color: '#2a2a4e' },
      horzLines: { color: '#2a2a4e' },
    },
    timeScale: {
      rightOffset: 10,
      barSpacing: 6,
      lockVisibleTimeRangeOnResize: true,
      rightBarStaysOnScroll: true,
      borderColor: '#2a2a4e',
    },
    rightPriceScale: {
      autoScale: true,
      scaleMargins: {
        top: 0,
        bottom: 0,
      },
      borderColor: '#2a2a4e',
    },
    leftPriceScale: {
      visible: false,
    },
    crosshair: {
      mode: LightweightCharts.CrosshairMode.Normal,
      vertLine: {
        color: '#e0e0e0',
        width: 1,
        style: LightweightCharts.LineStyle.Dashed,
        labelBackgroundColor: '#1a1a2e',
      },
      horzLine: {
        color: '#e0e0e0',
        width: 1,
        style: LightweightCharts.LineStyle.Dashed,
        labelBackgroundColor: '#1a1a2e',
      },
    },
    handleScroll: {
      vertTouchDrag: false,
    },
    handleScale: {
      axisPressedMouseMove: false,
    },
  };

  const chart = LightweightCharts.createChart(chartContainer, chartOptions);

  const candlestickSeries = chart.addSeries(LightweightCharts.CandlestickSeries, {
    upColor: '#26a69a',
    downColor: '#ef5350',
    borderVisible: false,
    wickUpColor: '#26a69a',
    wickDownColor: '#ef5350',
  });

  const SMA_PERIOD = 20;
  const candles = [];
  const smaData = [];

  const smaSeries = chart.addSeries(LightweightCharts.LineSeries, {
    color: '#2196F3',
    lineWidth: 2,
    priceLineVisible: false,
    crosshairMarkerVisible: false,
    lastValueVisible: false,
  });

  function calculateSMA(data, period) {
    if (data.length < period) {
      return null;
    }
    const slice = data.slice(-period);
    const sum = slice.reduce((acc, candle) => acc + candle.close, 0);
    return sum / period;
  }

  function updateCandle(candle) {
    const existingIndex = candles.findIndex(function(c) {
      return c.time === candle.time;
    });
    if (existingIndex >= 0) {
      candles[existingIndex] = candle;
    } else {
      candles.push(candle);
      candles.sort(function(a, b) {
        return a.time - b.time;
      });
    }

    candlestickSeries.update(candle);

    const smaValue = calculateSMA(candles, SMA_PERIOD);
    if (smaValue !== null) {
      const smaPoint = { time: candle.time, value: smaValue };
      smaSeries.update(smaPoint);

      const existingSmaIndex = smaData.findIndex(function(d) {
        return d.time === candle.time;
      });
      if (existingSmaIndex >= 0) {
        smaData[existingSmaIndex] = smaPoint;
      } else {
        smaData.push(smaPoint);
      }
    }
  }

  function setCandles(newCandles) {
    candles.length = 0;
    Array.prototype.push.apply(candles, newCandles);
    candlestickSeries.setData(candles);

    smaData.length = 0;
    for (let i = SMA_PERIOD - 1; i < candles.length; i++) {
      const slice = candles.slice(i - SMA_PERIOD + 1, i + 1);
      const sum = slice.reduce(function(acc, c) { return acc + c.close; }, 0);
      smaData.push({ time: candles[i].time, value: sum / SMA_PERIOD });
    }
    smaSeries.setData(smaData);
  }

  function resizeChart() {
    const width = chartContainer.clientWidth;
    const height = chartContainer.clientHeight;
    chart.applyOptions({ width, height });
  }

  window.addEventListener('resize', resizeChart);
  resizeChart();

  if (typeof module !== 'undefined' && module.exports) {
    module.exports = { chart, candlestickSeries, smaSeries, candles, smaData, updateCandle, setCandles };
  } else {
    global.ChartModule = { chart, candlestickSeries, smaSeries, candles, smaData, updateCandle, setCandles };
  }

})(typeof window !== 'undefined' ? window : global);
