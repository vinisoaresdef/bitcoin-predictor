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
      rightOffset: 8,
      barSpacing: 4,
      timeVisible: true,
      secondsVisible: true,
      lockVisibleTimeRangeOnResize: false,
      rightBarStaysOnScroll: false,
      borderColor: '#2a2a4e',
      fixLeftEdge: false,
      fixRightEdge: false,
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
      horzTouchDrag: true,
      mouseWheel: true,
      pressedMouseMove: true,
    },
    handleScale: {
      axisPressedMouseMove: true,
      mouseWheel: true,
      pinch: true,
    },
  };

  const chart = LightweightCharts.createChart(chartContainer, chartOptions);

  const realSeries = chart.addSeries(LightweightCharts.CandlestickSeries, {
    upColor: '#26a69a',
    downColor: '#ef5350',
    borderVisible: true,
    wickUpColor: '#26a69a',
    wickDownColor: '#ef5350',
  });

  const predictedSeries = chart.addSeries(LightweightCharts.CandlestickSeries, {
    upColor: 'rgba(38, 166, 154, 0.6)',
    downColor: 'rgba(239, 83, 80, 0.6)',
    borderVisible: true,
    wickUpColor: 'rgba(38, 166, 154, 0.6)',
    wickDownColor: 'rgba(239, 83, 80, 0.6)',
    priceScaleId: '',
  });

  const SMA_PERIOD = 20;
  const SR_PERIOD = 20;
  const candles = [];
  const smaData = [];
  const predictedSmaData = [];
  const predictedCandles = [];
  const predictionData = [];

  let supportBroken = true;
  let resistanceBroken = true;
  let currentSupport = null;
  let currentResistance = null;
  let supportBreakIndex = -Infinity;
  let resistanceBreakIndex = -Infinity;

  const smaSeries = chart.addSeries(LightweightCharts.LineSeries, {
    color: '#2196F3',
    lineWidth: 2,
    priceLineVisible: false,
    crosshairMarkerVisible: false,
    lastValueVisible: false,
  });

  const predictionSeries = chart.addSeries(LightweightCharts.LineSeries, {
    color: '#FF9800',
    lineWidth: 2,
    lineStyle: LightweightCharts.LineStyle.Dashed,
    priceLineVisible: false,
    crosshairMarkerVisible: false,
    lastValueVisible: false,
    visible: true,
  });

  const predictedSmaSeries = chart.addSeries(LightweightCharts.LineSeries, {
    color: 'rgba(255, 152, 0, 0.5)',
    lineWidth: 1,
    lineStyle: LightweightCharts.LineStyle.Dotted,
    priceLineVisible: false,
    crosshairMarkerVisible: false,
    lastValueVisible: false,
    visible: false,
  });

  const supportSeries = chart.addSeries(LightweightCharts.LineSeries, {
    color: 'rgba(38, 166, 154, 0.8)',
    lineWidth: 1,
    lineStyle: LightweightCharts.LineStyle.Dashed,
    priceLineVisible: false,
    crosshairMarkerVisible: false,
    lastValueVisible: false,
    visible: false,
  });

  const resistanceSeries = chart.addSeries(LightweightCharts.LineSeries, {
    color: 'rgba(239, 83, 80, 0.8)',
    lineWidth: 1,
    lineStyle: LightweightCharts.LineStyle.Dashed,
    priceLineVisible: false,
    crosshairMarkerVisible: false,
    lastValueVisible: false,
    visible: false,
  });

  const volumeSeries = chart.addSeries(LightweightCharts.HistogramSeries, {
    priceScaleId: 'volume',
    priceFormat: {
      type: 'volume',
    },
  });

  try {
    var volScale = chart.priceScale('volume');
    if (volScale) {
      volScale.applyOptions({
        scaleMargins: {
          top: 0.8,
          bottom: 0,
        },
        visible: true,
      });
    }
  } catch (e) {
    console.warn('Volume scale setup failed:', e);
  }


  const patternBullSeries = chart.addSeries(LightweightCharts.CandlestickSeries, {
    upColor: 'rgba(33, 150, 243, 0.6)',
    downColor: 'rgba(33, 150, 243, 0.6)',
    borderVisible: true,
    wickUpColor: 'rgba(33, 150, 243, 0.5)',
    wickDownColor: 'rgba(33, 150, 243, 0.5)',
    priceScaleId: '',
    visible: false,
  });

  const patternBearSeries = chart.addSeries(LightweightCharts.CandlestickSeries, {
    upColor: 'rgba(255, 235, 59, 0.6)',
    downColor: 'rgba(255, 235, 59, 0.6)',
    borderVisible: true,
    wickUpColor: 'rgba(255, 235, 59, 0.5)',
    wickDownColor: 'rgba(255, 235, 59, 0.5)',
    priceScaleId: '',
    visible: false,
  });
  function calculateSMA(data, period) {
    if (data.length < period) {
      return null;
    }
    const slice = data.slice(-period);
    const sum = slice.reduce((acc, candle) => acc + candle.close, 0);
    return sum / period;
  }

  function calculateSupportResistance() {
    if (candles.length < SR_PERIOD) {
      return;
    }

    const windowCandles = candles.slice(-SR_PERIOD);
    const lastCandle = windowCandles[windowCandles.length - 1];

    const support = windowCandles.reduce(function(min, c) {
      return c.low < min ? c.low : min;
    }, Infinity);

    const resistance = windowCandles.reduce(function(max, c) {
      return c.high > max ? c.high : max;
    }, -Infinity);

    if (!supportBroken && currentSupport !== null) {
      if (lastCandle.close < currentSupport) {
        supportBroken = true;
        currentSupport = null;
        supportBreakIndex = candles.length - 1;
        supportSeries.setData([]);
        supportSeries.applyOptions({ visible: false });
      }
    }

    if (!resistanceBroken && currentResistance !== null) {
      if (lastCandle.close > currentResistance) {
        resistanceBroken = true;
        currentResistance = null;
        resistanceBreakIndex = candles.length - 1;
        resistanceSeries.setData([]);
        resistanceSeries.applyOptions({ visible: false });
      }
    }

    if (supportBroken && support !== null && isFinite(support)) {
      if (candles.length - supportBreakIndex >= SR_PERIOD) {
        supportBroken = false;
        currentSupport = support;
        const firstTime = candles[0].time;
        const lastTime = lastCandle.time;
        supportSeries.setData([
          { time: firstTime, value: support },
          { time: lastTime, value: support }
        ]);
        supportSeries.applyOptions({ visible: true });
      }
    } else if (!supportBroken && currentSupport !== null) {
      const firstTime = candles[0].time;
      const lastTime = lastCandle.time;
      supportSeries.setData([
        { time: firstTime, value: currentSupport },
        { time: lastTime, value: currentSupport }
      ]);
    }

    if (resistanceBroken && resistance !== null && isFinite(resistance)) {
      if (candles.length - resistanceBreakIndex >= SR_PERIOD) {
        resistanceBroken = false;
        currentResistance = resistance;
        const firstTime = candles[0].time;
        const lastTime = lastCandle.time;
        resistanceSeries.setData([
          { time: firstTime, value: resistance },
          { time: lastTime, value: resistance }
        ]);
        resistanceSeries.applyOptions({ visible: true });
      }
    } else if (!resistanceBroken && currentResistance !== null) {
      const firstTime = candles[0].time;
      const lastTime = lastCandle.time;
      resistanceSeries.setData([
        { time: firstTime, value: currentResistance },
        { time: lastTime, value: currentResistance }
      ]);
    }
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

    realSeries.update(candle);

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

    const volumeColor = candle.close >= candle.open
      ? 'rgba(38, 166, 154, 0.5)'
      : 'rgba(239, 83, 80, 0.5)';

    volumeSeries.update({
      time: candle.time,
      value: candle.volume || 0,
      color: volumeColor
    });

    calculateSupportResistance();
  }

  function setCandles(newCandles) {
    supportBroken = true;
    resistanceBroken = true;
    currentSupport = null;
    currentResistance = null;
    supportBreakIndex = -Infinity;
    resistanceBreakIndex = -Infinity;
    candles.length = 0;
    Array.prototype.push.apply(candles, newCandles);
    realSeries.setData(candles);

    smaData.length = 0;
    for (let i = SMA_PERIOD - 1; i < candles.length; i++) {
      const slice = candles.slice(i - SMA_PERIOD + 1, i + 1);
      const sum = slice.reduce(function(acc, c) { return acc + c.close; }, 0);
      smaData.push({ time: candles[i].time, value: sum / SMA_PERIOD });
    }
    smaSeries.setData(smaData);

    predictedSmaData.length = 0;
    predictedSmaSeries.setData(predictedSmaData);

    const volumeData = [];
    for (let i = 0; i < candles.length; i++) {
      volumeData.push({
        time: candles[i].time,
        value: candles[i].volume || 0,
        color: candles[i].close >= candles[i].open
          ? 'rgba(38, 166, 154, 0.5)'
          : 'rgba(239, 83, 80, 0.5)'
      });
    }
    volumeSeries.setData(volumeData);

    calculateSupportResistance();
  }

  const PREDICTION_COLORS = {
    UP: 'rgba(38, 166, 154, 0.6)',
    DOWN: 'rgba(239, 83, 80, 0.6)',
    UNCERTAIN: 'rgba(128, 128, 128, 0.3)',
  };

  function updatePredictedSMA(time, value, direction) {
    if (typeof time !== 'number' || isNaN(time)) {
      console.warn('Invalid predicted SMA time:', time);
      return;
    }
    if (typeof value !== 'number' || isNaN(value)) {
      console.warn('Invalid predicted SMA value:', value);
      return;
    }

    const color = PREDICTION_COLORS[direction] || PREDICTION_COLORS.UNCERTAIN;
    predictedSmaSeries.applyOptions({ color: color });

    predictedSmaData.length = 0;

    if (smaData.length > 0) {
      var lastSma = smaData[smaData.length - 1];
      predictedSmaData.push({ time: lastSma.time, value: lastSma.value });
    }

    predictedSmaData.push({ time: time, value: value });

    predictedSmaSeries.setData(predictedSmaData);
    predictedSmaSeries.applyOptions({ visible: true });
  }

  function updatePrediction(point) {
    const existingIndex = predictionData.findIndex(function(d) {
      return d.time === point.time;
    });
    if (existingIndex >= 0) {
      predictionData[existingIndex] = point;
    } else {
      predictionData.push(point);
      predictionData.sort(function(a, b) {
        return a.time - b.time;
      });
    }
    predictionSeries.update(point);
  }

  function clearPredictions() {
    predictionData.length = 0;
    predictionSeries.setData([]);
    patternBullSeries.setData([]);
    patternBullSeries.applyOptions({ visible: false });
    patternBearSeries.setData([]);
    patternBearSeries.applyOptions({ visible: false });
  }

  function clearPredictedCandles() {
    predictedCandles.length = 0;
    predictedSeries.setData([]);
  }

  function setPredictions(data) {
    predictionData.length = 0;
    Array.prototype.push.apply(predictionData, data);
    predictionData.sort(function(a, b) {
      return a.time - b.time;
    });
    predictionSeries.setData(predictionData);
  }

  function setPredictionVisible(visible) {
    predictionSeries.applyOptions({ visible: visible });
  }

  function updatePredictedCandle(candle, direction) {
    let upColor, downColor, wickUpColor, wickDownColor;

    switch (direction) {
      case 'UP':
        upColor = 'rgba(38, 166, 154, 0.6)';
        downColor = 'rgba(38, 166, 154, 0.6)';
        wickUpColor = 'rgba(38, 166, 154, 0.6)';
        wickDownColor = 'rgba(38, 166, 154, 0.6)';
        break;
      case 'DOWN':
        upColor = 'rgba(239, 83, 80, 0.6)';
        downColor = 'rgba(239, 83, 80, 0.6)';
        wickUpColor = 'rgba(239, 83, 80, 0.6)';
        wickDownColor = 'rgba(239, 83, 80, 0.6)';
        break;
      case 'UNCERTAIN':
      default:
        upColor = 'rgba(128, 128, 128, 0.3)';
        downColor = 'rgba(128, 128, 128, 0.3)';
        wickUpColor = 'rgba(128, 128, 128, 0.3)';
        wickDownColor = 'rgba(128, 128, 128, 0.3)';
        break;
    }

    predictedSeries.applyOptions({
      upColor: upColor,
      downColor: downColor,
      wickUpColor: wickUpColor,
      wickDownColor: wickDownColor,
      borderUpColor: upColor,
      borderDownColor: downColor,
      borderVisible: true,
    });

    predictedCandles.length = 0;

    const lastRealCandle = candles.length > 0 ? candles[candles.length - 1] : null;
    if (lastRealCandle) {
      predictedCandles.push({ time: lastRealCandle.time });
    }

    predictedCandles.push(candle);
    predictedSeries.setData(predictedCandles);
  }

  function resizeChart() {
    const width = chartContainer.clientWidth;
    const height = chartContainer.clientHeight;
    chart.applyOptions({ width, height });
  }

  function clearChart() {
    candles.length = 0;
    smaData.length = 0;
    predictedSmaData.length = 0;
    predictedCandles.length = 0;
    predictionData.length = 0;

    realSeries.setData([]);
    predictedSeries.setData([]);
    smaSeries.setData([]);
    predictedSmaSeries.setData([]);
    predictionSeries.setData([]);
    patternBullSeries.setData([]);
    patternBullSeries.applyOptions({ visible: false });
    patternBearSeries.setData([]);
    patternBearSeries.applyOptions({ visible: false });
    supportSeries.setData([]);
    volumeSeries.setData([]);
    supportSeries.applyOptions({ visible: false });
    resistanceSeries.setData([]);
    resistanceSeries.applyOptions({ visible: false });
    supportBroken = true;
    resistanceBroken = true;
    currentSupport = null;
    currentResistance = null;
    supportBreakIndex = -Infinity;
    resistanceBreakIndex = -Infinity;
  }

  function resetChart(newBarSpacing) {
    candles.length = 0;
    smaData.length = 0;
    predictedSmaData.length = 0;
    predictedCandles.length = 0;
    predictionData.length = 0;

    realSeries.setData([]);
    predictedSeries.setData([]);
    smaSeries.setData([]);
    predictedSmaSeries.setData([]);
    predictionSeries.setData([]);
    patternBullSeries.setData([]);
    patternBullSeries.applyOptions({ visible: false });
    patternBearSeries.setData([]);
    patternBearSeries.applyOptions({ visible: false });
    supportSeries.setData([]);
    supportSeries.applyOptions({ visible: false });
    resistanceSeries.setData([]);
    resistanceSeries.applyOptions({ visible: false });
    volumeSeries.setData([]);
    supportBroken = true;
    resistanceBroken = true;
    currentSupport = null;
    currentResistance = null;
    supportBreakIndex = -Infinity;
    resistanceBreakIndex = -Infinity;

    if (newBarSpacing) {
      chart.timeScale().applyOptions({ barSpacing: newBarSpacing });
    }
    chart.timeScale().resetTimeScale();
  }

  var resizeObserver = new ResizeObserver(function() {
    resizeChart();
  });
  resizeObserver.observe(chartContainer);
  resizeChart();


  function updatePatternPrediction(candlesArr, prediction) {
    if (!prediction || prediction.count === 0) {
      patternBullSeries.setData([]);
      patternBullSeries.applyOptions({ visible: false });
      patternBearSeries.setData([]);
      patternBearSeries.applyOptions({ visible: false });
      return;
    }

    if (!candlesArr || candlesArr.length === 0) return;

    var lastCandle = candlesArr[candlesArr.length - 1];
    var lastTime = lastCandle.time;
    var avgPrice = (lastCandle.high + lastCandle.low) / 2;
    var candleRange = lastCandle.high - lastCandle.low;
    var candleBody = Math.abs(lastCandle.close - lastCandle.open);

    var predictionOffset = 3;

    var predictedCandles = [];
    var isBullish = prediction.predicts === 'bullish';

    for (var i = 0; i < prediction.count; i++) {
        var futureTime = lastTime + (i + 1) * predictionOffset;
        var bodySize = candleBody * 0.8;
        var wickSize = candleRange * 0.3;

        if (isBullish) {
            predictedCandles.push({
                time: futureTime,
                open: avgPrice - bodySize * 0.3,
                high: avgPrice + bodySize * 0.7 + wickSize,
                low: avgPrice - bodySize * 0.3 - wickSize,
                close: avgPrice + bodySize * 0.7
            });
        } else {
            predictedCandles.push({
                time: futureTime,
                open: avgPrice + bodySize * 0.3,
                high: avgPrice + bodySize * 0.7 + wickSize,
                low: avgPrice - bodySize * 0.3 - wickSize,
                close: avgPrice - bodySize * 0.7
            });
        }
    }

    if (isBullish) {
        patternBullSeries.setData(predictedCandles);
        patternBullSeries.applyOptions({ visible: true });
        patternBearSeries.setData([]);
        patternBearSeries.applyOptions({ visible: false });
    } else {
        patternBearSeries.setData(predictedCandles);
        patternBearSeries.applyOptions({ visible: true });
        patternBullSeries.setData([]);
        patternBullSeries.applyOptions({ visible: false });
    }
  }

  function setPatternPredictionVisible(visible) {
    if (!visible) {
      patternBullSeries.setData([]);
      patternBullSeries.applyOptions({ visible: false });
      patternBearSeries.setData([]);
      patternBearSeries.applyOptions({ visible: false });
    }
  }

  function centerLastCandle() {
    if (candles.length === 0) return;
    var visibleRange = chart.timeScale().getVisibleLogicalRange();
    if (!visibleRange) {
      chart.timeScale().fitContent();
      return;
    }
    var viewportWidth = visibleRange.to - visibleRange.from;
    var lastIndex = candles.length - 1;
    var halfViewport = viewportWidth / 2;
    chart.timeScale().setVisibleLogicalRange({
      from: lastIndex - halfViewport,
      to: lastIndex + halfViewport
    });
  }

  if (typeof module !== 'undefined' && module.exports) {
    module.exports = {
      chart,
      realSeries,
      predictedSeries,
      candlestickSeries: realSeries,
      smaSeries,
      predictionSeries,
      predictedSmaSeries,
      supportSeries,
      resistanceSeries,
      volumeSeries,
      candles,
      smaData,
      predictedSmaData,
      predictedCandles,
      predictionData,
      updateCandle,
      setCandles,
      updatePredictedSMA,
      updatePrediction,
      clearPredictions,
      clearPredictedCandles,
      clearChart,
      patternBullSeries,
      patternBearSeries,
      updatePatternPrediction,
      setPatternPredictionVisible,
      centerLastCandle,
      resetChart,
      setPredictions,
      setPredictionVisible,
      updatePredictedCandle,
    };
  } else {
    global.ChartModule = {
      chart,
      realSeries,
      predictedSeries,
      candlestickSeries: realSeries,
      smaSeries,
      predictionSeries,
      predictedSmaSeries,
      supportSeries,
      resistanceSeries,
      volumeSeries,
      candles,
      smaData,
      predictedSmaData,
      predictedCandles,
      predictionData,
      updateCandle,
      setCandles,
      updatePredictedSMA,
      updatePrediction,
      clearPredictions,
      clearPredictedCandles,
      clearChart,
      patternBullSeries,
      patternBearSeries,
      updatePatternPrediction,
      setPatternPredictionVisible,
      centerLastCandle,
      resetChart,
      setPredictions,
      setPredictionVisible,
      updatePredictedCandle,
    };
  }

})(typeof window !== 'undefined' ? window : global);
