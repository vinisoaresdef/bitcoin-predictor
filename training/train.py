"""LightGBM model training with walk-forward validation."""

import argparse
import json
import sys
from pathlib import Path
from typing import Any, Dict, List, Tuple

import joblib
import numpy as np
import pandas as pd
from lightgbm import LGBMClassifier
from sklearn.metrics import (
    accuracy_score,
    confusion_matrix,
    f1_score,
    precision_score,
    recall_score,
)

FEATURE_COLUMNS = [
    'returns',
    'log_returns',
    'high_low_range',
    'body_size',
    'upper_shadow',
    'lower_shadow',
    'volume',
    'volume_change',
    'sma_5',
    'sma_10',
    'sma_20',
    'sma_5_distance',
    'sma_10_distance',
    'sma_20_distance',
    'rsi',
    'macd_histogram',
    'bb_percent_b',
    'atr',
    'price_momentum',
    'price_acceleration',
    'hour_sin',
    'day_of_week_sin',
]

LABELS = ['DOWN', 'UNCERTAIN', 'UP']
LABEL_TO_INT = {label: i for i, label in enumerate(LABELS)}


def load_labeled_data(input_path: str) -> pd.DataFrame:
    """Load labeled data from parquet file."""
    input_file = Path(input_path)
    if not input_file.exists():
        raise FileNotFoundError(f"Input file not found: {input_path}")
    
    df = pd.read_parquet(input_path)
    
    missing_features = set(FEATURE_COLUMNS) - set(df.columns)
    if missing_features:
        raise ValueError(f"Missing feature columns: {missing_features}")
    
    if 'label' not in df.columns:
        raise ValueError("Missing 'label' column in data")
    
    return df


def create_walk_forward_splits(
    data: pd.DataFrame,
    n_splits: int = 5
) -> List[Tuple[np.ndarray, np.ndarray]]:
    """Create walk-forward validation splits using TimeSeriesSplit."""
    from sklearn.model_selection import TimeSeriesSplit
    
    tscv = TimeSeriesSplit(n_splits=n_splits)
    splits = []
    for train_idx, test_idx in tscv.split(data):
        splits.append((train_idx, test_idx))
    
    return splits


def train_lightgbm_model(
    X_train: pd.DataFrame,
    y_train: np.ndarray,
    feature_names: List[str]
) -> LGBMClassifier:
    """Train a LightGBM classifier."""
    model = LGBMClassifier(
        objective='multiclass',
        metric='multi_logloss',
        num_class=3,
        learning_rate=0.05,
        n_estimators=100,
        max_depth=6,
        num_leaves=31,
        verbose=-1,
    )
    model.fit(X_train, y_train)
    return model


def evaluate_model(
    model: LGBMClassifier,
    X_test: pd.DataFrame,
    y_test: np.ndarray
) -> Dict[str, Any]:
    """Evaluate model on test set."""
    y_pred = model.predict(X_test)
    
    metrics = {
        'accuracy': float(accuracy_score(y_test, y_pred)),
        'precision_macro': float(precision_score(y_test, y_pred, average='macro', zero_division=0)),
        'recall_macro': float(recall_score(y_test, y_pred, average='macro', zero_division=0)),
        'f1_macro': float(f1_score(y_test, y_pred, average='macro', zero_division=0)),
    }
    
    precision_per_class = precision_score(y_test, y_pred, average=None, zero_division=0)
    recall_per_class = recall_score(y_test, y_pred, average=None, zero_division=0)
    f1_per_class = f1_score(y_test, y_pred, average=None, zero_division=0)
    
    for i, label in enumerate(LABELS):
        metrics[f'precision_{label.lower()}'] = float(precision_per_class[i])
        metrics[f'recall_{label.lower()}'] = float(recall_per_class[i])
        metrics[f'f1_{label.lower()}'] = float(f1_per_class[i])
    
    cm = confusion_matrix(y_test, y_pred, labels=[0, 1, 2])
    metrics['confusion_matrix'] = cm.tolist()
    
    return metrics


def train_with_cross_validation(
    data: pd.DataFrame,
    n_splits: int = 5
) -> Tuple[LGBMClassifier, Dict[str, Any]]:
    """Train models using walk-forward validation and return best model."""
    X = data[FEATURE_COLUMNS]
    y = data['label'].map(LABEL_TO_INT).values
    
    splits = create_walk_forward_splits(data, n_splits=n_splits)
    
    fold_metrics = []
    models = []
    
    print(f"Training with {n_splits} walk-forward splits...")
    
    for fold_idx, (train_idx, test_idx) in enumerate(splits):
        print(f"\nFold {fold_idx + 1}/{n_splits}:")
        print(f"  Train: {len(train_idx)} samples")
        print(f"  Test: {len(test_idx)} samples")
        
        X_train, X_test = X.iloc[train_idx], X.iloc[test_idx]
        y_train, y_test = y[train_idx], y[test_idx]
        
        model = train_lightgbm_model(X_train, y_train, FEATURE_COLUMNS)
        models.append(model)
        
        metrics = evaluate_model(model, X_test, y_test)
        fold_metrics.append(metrics)
        
        print(f"  Accuracy: {metrics['accuracy']:.4f}")
        print(f"  Precision (macro): {metrics['precision_macro']:.4f}")
        print(f"  Recall (macro): {metrics['recall_macro']:.4f}")
        print(f"  F1 (macro): {metrics['f1_macro']:.4f}")
    
    best_fold_idx = np.argmax([m['f1_macro'] for m in fold_metrics])
    best_model = models[best_fold_idx]
    
    print(f"\nBest model from fold {best_fold_idx + 1} (F1: {fold_metrics[best_fold_idx]['f1_macro']:.4f})")
    
    aggregated_metrics = {
        'best_fold': int(best_fold_idx + 1),
        'n_splits': int(n_splits),
        'fold_metrics': fold_metrics,
        'mean_metrics': {
            'accuracy': float(np.mean([m['accuracy'] for m in fold_metrics])),
            'precision_macro': float(np.mean([m['precision_macro'] for m in fold_metrics])),
            'recall_macro': float(np.mean([m['recall_macro'] for m in fold_metrics])),
            'f1_macro': float(np.mean([m['f1_macro'] for m in fold_metrics])),
        },
        'std_metrics': {
            'accuracy': float(np.std([m['accuracy'] for m in fold_metrics])),
            'precision_macro': float(np.std([m['precision_macro'] for m in fold_metrics])),
            'recall_macro': float(np.std([m['recall_macro'] for m in fold_metrics])),
            'f1_macro': float(np.std([m['f1_macro'] for m in fold_metrics])),
        },
        'best_fold_metrics': fold_metrics[best_fold_idx],
        'feature_names': FEATURE_COLUMNS,
        'n_features': len(FEATURE_COLUMNS),
        'n_samples': len(data),
        'class_distribution': {k: int(v) for k, v in data['label'].value_counts().items()},
    }
    
    return best_model, aggregated_metrics


def save_model(model: LGBMClassifier, output_path: str) -> None:
    """Save trained model using joblib."""
    output_file = Path(output_path)
    output_file.parent.mkdir(parents=True, exist_ok=True)
    joblib.dump(model, output_path)


def save_metrics(metrics: Dict[str, Any], output_path: str) -> None:
    """Save metrics to JSON file."""
    output_file = Path(output_path)
    output_file.parent.mkdir(parents=True, exist_ok=True)
    
    with open(output_path, 'w') as f:
        json.dump(metrics, f, indent=2)


def main():
    """Main entry point for CLI."""
    parser = argparse.ArgumentParser(
        description='Train LightGBM classifier with walk-forward validation'
    )
    parser.add_argument(
        '--input',
        type=str,
        default='training/data/labeled.parquet',
        help='Input labeled parquet file path (default: training/data/labeled.parquet)'
    )
    parser.add_argument(
        '--output',
        type=str,
        default='training/models/',
        help='Output directory path (default: training/models/)'
    )
    parser.add_argument(
        '--n-splits',
        type=int,
        default=5,
        help='Number of walk-forward splits (default: 5)'
    )
    
    args = parser.parse_args()
    
    try:
        print(f"Loading labeled data from {args.input}...")
        data = load_labeled_data(args.input)
        print(f"Loaded {len(data)} rows with {len(FEATURE_COLUMNS)} features")
        
        label_counts = data['label'].value_counts()
        print("\nLabel distribution:")
        for label, count in label_counts.items():
            pct = count / len(data) * 100
            print(f"  {label}: {count} ({pct:.1f}%)")
        
        best_model, metrics = train_with_cross_validation(data, n_splits=args.n_splits)
        
        print("\n" + "="*50)
        print("AGGREGATED RESULTS (mean ± std)")
        print("="*50)
        mean = metrics['mean_metrics']
        std = metrics['std_metrics']
        print(f"Accuracy:      {mean['accuracy']:.4f} ± {std['accuracy']:.4f}")
        print(f"Precision:     {mean['precision_macro']:.4f} ± {std['precision_macro']:.4f}")
        print(f"Recall:        {mean['recall_macro']:.4f} ± {std['recall_macro']:.4f}")
        print(f"F1 (macro):    {mean['f1_macro']:.4f} ± {std['f1_macro']:.4f}")
        
        output_dir = Path(args.output)
        output_dir.mkdir(parents=True, exist_ok=True)
        
        model_path = output_dir / 'model.joblib'
        print(f"\nSaving best model to {model_path}...")
        save_model(best_model, str(model_path))
        
        metrics_path = output_dir / 'metrics.json'
        print(f"Saving metrics to {metrics_path}...")
        save_metrics(metrics, str(metrics_path))
        
        print("\nTraining complete!")
        return 0
        
    except FileNotFoundError as e:
        print(f"Error: {e}", file=sys.stderr)
        return 1
    except ValueError as e:
        print(f"Error: {e}", file=sys.stderr)
        return 1
    except Exception as e:
        print(f"Error during training: {e}", file=sys.stderr)
        import traceback
        traceback.print_exc()
        return 1


if __name__ == '__main__':
    sys.exit(main())
