(function(global) {
  'use strict';

  function body(c) { return Math.abs(c.close - c.open); }
  function range(c) { return c.high - c.low; }
  function upperWick(c) { return c.high - Math.max(c.open, c.close); }
  function lowerWick(c) { return Math.min(c.open, c.close) - c.low; }
  function isGreen(c) { return c.close > c.open; }
  function isRed(c) { return c.close < c.open; }
  function isDoji(c) { return body(c) < range(c) * 0.05 && range(c) > 0; }
  function bodyRatio(c) { return range(c) > 0 ? body(c) / range(c) : 0; }

  var PATTERN = { BULLISH: 'bullish', BEARISH: 'bearish', NEUTRAL: 'neutral' };

  function detectHammer(candles) {
    var c = candles[candles.length - 1];
    var b = body(c), lw = lowerWick(c), uw = upperWick(c);
    return lw > b * 2 && uw < b * 0.3 && b > 0;
  }

  function detectInvertedHammer(candles) {
    var c = candles[candles.length - 1];
    var b = body(c), uw = upperWick(c);
    return uw > b * 2 && lowerWick(c) < b * 0.3 && b > 0;
  }

  function detectShootingStar(candles) {
    var c = candles[candles.length - 1];
    var b = body(c), uw = upperWick(c);
    return uw > b * 2 && lowerWick(c) < b * 0.3 && b > 0;
  }

  function detectHangingMan(candles) {
    if (candles.length < 5) return false;
    var c = candles[candles.length - 1];
    return lowerWick(c) > body(c) * 2 && upperWick(c) < body(c) * 0.3;
  }

  function detectDoji(candles) { return isDoji(candles[candles.length - 1]); }
  function detectDragonflyDoji(candles) {
    var c = candles[candles.length - 1];
    return isDoji(c) && lowerWick(c) > body(c) * 3;
  }
  function detectGravestoneDoji(candles) {
    var c = candles[candles.length - 1];
    return isDoji(c) && upperWick(c) > body(c) * 3;
  }
  function detectMarubozu(candles) {
    return bodyRatio(candles[candles.length - 1]) > 0.85;
  }
  function detectSpinningTop(candles) {
    var c = candles[candles.length - 1];
    return bodyRatio(c) < 0.3 && range(c) > 0;
  }

  function detectBullishEngulfing(candles) {
    if (candles.length < 2) return false;
    var p = candles[candles.length - 2], c = candles[candles.length - 1];
    return isRed(p) && isGreen(c) && c.open <= p.close && c.close >= p.open;
  }

  function detectBearishEngulfing(candles) {
    if (candles.length < 2) return false;
    var p = candles[candles.length - 2], c = candles[candles.length - 1];
    return isGreen(p) && isRed(c) && c.open >= p.close && c.close <= p.open;
  }

  function detectBullishHarami(candles) {
    if (candles.length < 2) return false;
    var p = candles[candles.length - 2], c = candles[candles.length - 1];
    return isRed(p) && c.high <= p.high && c.low >= p.low && body(c) < body(p) * 0.6;
  }

  function detectBearishHarami(candles) {
    if (candles.length < 2) return false;
    var p = candles[candles.length - 2], c = candles[candles.length - 1];
    return isGreen(p) && c.high <= p.high && c.low >= p.low && body(c) < body(p) * 0.6;
  }

  function detectPiercingLine(candles) {
    if (candles.length < 2) return false;
    var p = candles[candles.length - 2], c = candles[candles.length - 1];
    return isRed(p) && isGreen(c) && c.open < p.close && c.close > (p.open + p.close) / 2 && c.close < p.open;
  }

  function detectDarkCloudCover(candles) {
    if (candles.length < 2) return false;
    var p = candles[candles.length - 2], c = candles[candles.length - 1];
    return isGreen(p) && isRed(c) && c.open > p.close && c.close < (p.open + p.close) / 2 && c.close > p.open;
  }

  function detectTweezerBottom(candles) {
    if (candles.length < 2) return false;
    var p = candles[candles.length - 2], c = candles[candles.length - 1];
    return Math.abs(p.low - c.low) < range(c) * 0.05;
  }

  function detectTweezerTop(candles) {
    if (candles.length < 2) return false;
    var p = candles[candles.length - 2], c = candles[candles.length - 1];
    return Math.abs(p.high - c.high) < range(c) * 0.05;
  }

  function detectMorningStar(candles) {
    if (candles.length < 3) return false;
    var f = candles[candles.length - 3], m = candles[candles.length - 2], l = candles[candles.length - 1];
    return isRed(f) && body(m) < body(f) * 0.3 && isGreen(l) && l.close > (f.open + f.close) / 2;
  }

  function detectEveningStar(candles) {
    if (candles.length < 3) return false;
    var f = candles[candles.length - 3], m = candles[candles.length - 2], l = candles[candles.length - 1];
    return isGreen(f) && body(m) < body(f) * 0.3 && isRed(l) && l.close < (f.open + f.close) / 2;
  }

  function detectThreeWhiteSoldiers(candles) {
    if (candles.length < 3) return false;
    var c1 = candles[candles.length - 3], c2 = candles[candles.length - 2], c3 = candles[candles.length - 1];
    return isGreen(c1) && isGreen(c2) && isGreen(c3) && c2.close > c1.close && c3.close > c2.close && c2.open > c1.open && c3.open > c2.open;
  }

  function detectThreeBlackCrows(candles) {
    if (candles.length < 3) return false;
    var c1 = candles[candles.length - 3], c2 = candles[candles.length - 2], c3 = candles[candles.length - 1];
    return isRed(c1) && isRed(c2) && isRed(c3) && c2.close < c1.close && c3.close < c2.close && c2.open < c1.open && c3.open < c2.open;
  }

  var PATTERNS = [
    { name: 'Hammer', type: 'bullish_reversal', detect: detectHammer, predicts: PATTERN.BULLISH, count: 1 },
    { name: 'Inverted Hammer', type: 'bullish_reversal', detect: detectInvertedHammer, predicts: PATTERN.BULLISH, count: 1 },
    { name: 'Bullish Engulfing', type: 'bullish_reversal', detect: detectBullishEngulfing, predicts: PATTERN.BULLISH, count: 2 },
    { name: 'Piercing Line', type: 'bullish_reversal', detect: detectPiercingLine, predicts: PATTERN.BULLISH, count: 1 },
    { name: 'Morning Star', type: 'bullish_reversal', detect: detectMorningStar, predicts: PATTERN.BULLISH, count: 2 },
    { name: 'Three White Soldiers', type: 'bullish_reversal', detect: detectThreeWhiteSoldiers, predicts: PATTERN.BULLISH, count: 1 },
    { name: 'Bullish Harami', type: 'bullish_reversal', detect: detectBullishHarami, predicts: PATTERN.BULLISH, count: 1 },
    { name: 'Tweezer Bottom', type: 'bullish_reversal', detect: detectTweezerBottom, predicts: PATTERN.BULLISH, count: 1 },
    { name: 'Dragonfly Doji', type: 'bullish_reversal', detect: detectDragonflyDoji, predicts: PATTERN.BULLISH, count: 1 },
    { name: 'Shooting Star', type: 'bearish_reversal', detect: detectShootingStar, predicts: PATTERN.BEARISH, count: 1 },
    { name: 'Hanging Man', type: 'bearish_reversal', detect: detectHangingMan, predicts: PATTERN.BEARISH, count: 1 },
    { name: 'Bearish Engulfing', type: 'bearish_reversal', detect: detectBearishEngulfing, predicts: PATTERN.BEARISH, count: 2 },
    { name: 'Dark Cloud Cover', type: 'bearish_reversal', detect: detectDarkCloudCover, predicts: PATTERN.BEARISH, count: 1 },
    { name: 'Evening Star', type: 'bearish_reversal', detect: detectEveningStar, predicts: PATTERN.BEARISH, count: 2 },
    { name: 'Three Black Crows', type: 'bearish_reversal', detect: detectThreeBlackCrows, predicts: PATTERN.BEARISH, count: 1 },
    { name: 'Bearish Harami', type: 'bearish_reversal', detect: detectBearishHarami, predicts: PATTERN.BEARISH, count: 1 },
    { name: 'Tweezer Top', type: 'bearish_reversal', detect: detectTweezerTop, predicts: PATTERN.BEARISH, count: 1 },
    { name: 'Gravestone Doji', type: 'bearish_reversal', detect: detectGravestoneDoji, predicts: PATTERN.BEARISH, count: 1 },
    { name: 'Doji', type: 'indecision', detect: detectDoji, predicts: PATTERN.NEUTRAL, count: 0 },
    { name: 'Spinning Top', type: 'indecision', detect: detectSpinningTop, predicts: PATTERN.NEUTRAL, count: 0 },
    { name: 'Marubozu', type: 'continuation', detect: detectMarubozu, predicts: PATTERN.NEUTRAL, count: 0 },
  ];

  function analyzePatterns(candles) {
    if (!candles || candles.length < 2) return null;
    for (var i = 0; i < PATTERNS.length; i++) {
      var p = PATTERNS[i];
      if (p.detect(candles)) {
        return { name: p.name, type: p.type, predicts: p.predicts, count: p.count };
      }
    }
    var last = candles[candles.length - 1];
    if (isGreen(last)) return { name: 'Bullish', type: 'continuation', predicts: PATTERN.BULLISH, count: 1 };
    if (isRed(last)) return { name: 'Bearish', type: 'continuation', predicts: PATTERN.BEARISH, count: 1 };
    return null;
  }

  global.PatternEngine = { analyze: analyzePatterns, PATTERN: PATTERN };

})(typeof window !== 'undefined' ? window : global);
