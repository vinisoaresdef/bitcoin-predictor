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

  function resizeChart() {
    const width = chartContainer.clientWidth;
    const height = chartContainer.clientHeight;
    chart.applyOptions({ width, height });
  }

  window.addEventListener('resize', resizeChart);
  resizeChart();

  if (typeof module !== 'undefined' && module.exports) {
    module.exports = { chart, candlestickSeries };
  } else {
    global.ChartModule = { chart, candlestickSeries };
  }

})(typeof window !== 'undefined' ? window : global);
