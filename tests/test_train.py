"""Tests for training/train.py."""

import json
import os
import sys
from pathlib import Path

import joblib
import numpy as np
import pandas as pd
import pytest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..', 'training'))

from train import (
    load_labeled_data,
    create_walk_forward_splits,
    train_lightgbm_model,
    evaluate_model,
    train_with_cross_validation,
    save_model,
    save_metrics,
    main,
    FEATURE_COLUMNS,
    LABELS,
)


@pytest.fixture
def sample_labeled_data():
    """Create sample labeled data for testing."""
    np.random.seed(42)
    n_rows = 200
    
    features = {}
    for col in FEATURE_COLUMNS:
        if 'sin' in col:
            features[col] = np.random.uniform(-1, 1, n_rows)
        elif col in ['volume']:
            features[col] = np.random.uniform(100, 500, n_rows)
        elif col in ['body_size', 'upper_shadow', 'lower_shadow', 'bb_percent_b']:
            features[col] = np.random.uniform(0, 1, n_rows)
        else:
            features[col] = np.random.randn(n_rows) * 0.01
    
    df = pd.DataFrame(features)
    df['label'] = np.random.choice(['UP', 'DOWN', 'UNCERTAIN'], n_rows)
    
    return df


@pytest.fixture
def sample_labeled_file(tmp_path, sample_labeled_data):
    """Create a sample labeled parquet file."""
    file_path = tmp_path / "labeled.parquet"
    sample_labeled_data.to_parquet(file_path, index=False)
    return str(file_path)


class TestModelFileCreated:
    """Test that model file is created correctly."""
    
    def test_model_file_created(self, tmp_path, sample_labeled_file):
        """Verify joblib file is created after training."""
        output_dir = tmp_path / "models"
        
        from unittest.mock import patch
        with patch('sys.argv', [
            'train.py',
            '--input', sample_labeled_file,
            '--output', str(output_dir),
            '--n-splits', '3'
        ]):
            exit_code = main()
        
        assert exit_code == 0, "Training should complete successfully"
        
        model_path = output_dir / 'model.joblib'
        assert model_path.exists(), "Model file should be created"
        
        model = joblib.load(model_path)
        assert hasattr(model, 'predict'), "Loaded model should have predict method"
    
    def test_model_file_is_joblib_format(self, tmp_path, sample_labeled_file):
        """Verify saved model is in joblib format and can be loaded."""
        output_dir = tmp_path / "models"
        
        from unittest.mock import patch
        with patch('sys.argv', [
            'train.py',
            '--input', sample_labeled_file,
            '--output', str(output_dir),
            '--n-splits', '3'
        ]):
            main()
        
        model_path = output_dir / 'model.joblib'
        
        model = joblib.load(model_path)
        assert model is not None
    
    def test_model_directory_created(self, tmp_path, sample_labeled_file):
        """Verify output directory is created if it doesn't exist."""
        nested_output = tmp_path / "nested" / "models"
        
        from unittest.mock import patch
        with patch('sys.argv', [
            'train.py',
            '--input', sample_labeled_file,
            '--output', str(nested_output),
            '--n-splits', '3'
        ]):
            main()
        
        assert nested_output.exists(), "Nested directory should be created"
        assert (nested_output / 'model.joblib').exists()


class TestMetricsCalculated:
    """Test that metrics are calculated and saved correctly."""
    
    def test_metrics_file_created(self, tmp_path, sample_labeled_file):
        """Verify metrics.json is created."""
        output_dir = tmp_path / "models"
        
        from unittest.mock import patch
        with patch('sys.argv', [
            'train.py',
            '--input', sample_labeled_file,
            '--output', str(output_dir),
            '--n-splits', '3'
        ]):
            main()
        
        metrics_path = output_dir / 'metrics.json'
        assert metrics_path.exists(), "Metrics file should be created"
    
    def test_metrics_contains_accuracy(self, tmp_path, sample_labeled_file):
        """Verify metrics contain accuracy score."""
        output_dir = tmp_path / "models"
        
        from unittest.mock import patch
        with patch('sys.argv', [
            'train.py',
            '--input', sample_labeled_file,
            '--output', str(output_dir),
            '--n-splits', '3'
        ]):
            main()
        
        metrics_path = output_dir / 'metrics.json'
        with open(metrics_path) as f:
            metrics = json.load(f)
        
        assert 'mean_metrics' in metrics
        assert 'accuracy' in metrics['mean_metrics']
        assert isinstance(metrics['mean_metrics']['accuracy'], float)
        assert 0 <= metrics['mean_metrics']['accuracy'] <= 1
    
    def test_metrics_contains_precision(self, tmp_path, sample_labeled_file):
        """Verify metrics contain precision score."""
        output_dir = tmp_path / "models"
        
        from unittest.mock import patch
        with patch('sys.argv', [
            'train.py',
            '--input', sample_labeled_file,
            '--output', str(output_dir),
            '--n-splits', '3'
        ]):
            main()
        
        metrics_path = output_dir / 'metrics.json'
        with open(metrics_path) as f:
            metrics = json.load(f)
        
        assert 'precision_macro' in metrics['mean_metrics']
        assert isinstance(metrics['mean_metrics']['precision_macro'], float)
    
    def test_metrics_contains_recall(self, tmp_path, sample_labeled_file):
        """Verify metrics contain recall score."""
        output_dir = tmp_path / "models"
        
        from unittest.mock import patch
        with patch('sys.argv', [
            'train.py',
            '--input', sample_labeled_file,
            '--output', str(output_dir),
            '--n-splits', '3'
        ]):
            main()
        
        metrics_path = output_dir / 'metrics.json'
        with open(metrics_path) as f:
            metrics = json.load(f)
        
        assert 'recall_macro' in metrics['mean_metrics']
        assert isinstance(metrics['mean_metrics']['recall_macro'], float)
    
    def test_metrics_contains_f1(self, tmp_path, sample_labeled_file):
        """Verify metrics contain F1 score."""
        output_dir = tmp_path / "models"
        
        from unittest.mock import patch
        with patch('sys.argv', [
            'train.py',
            '--input', sample_labeled_file,
            '--output', str(output_dir),
            '--n-splits', '3'
        ]):
            main()
        
        metrics_path = output_dir / 'metrics.json'
        with open(metrics_path) as f:
            metrics = json.load(f)
        
        assert 'f1_macro' in metrics['mean_metrics']
        assert isinstance(metrics['mean_metrics']['f1_macro'], float)
    
    def test_metrics_contains_fold_metrics(self, tmp_path, sample_labeled_file):
        """Verify metrics contain per-fold results."""
        output_dir = tmp_path / "models"
        
        from unittest.mock import patch
        with patch('sys.argv', [
            'train.py',
            '--input', sample_labeled_file,
            '--output', str(output_dir),
            '--n-splits', '3'
        ]):
            main()
        
        metrics_path = output_dir / 'metrics.json'
        with open(metrics_path) as f:
            metrics = json.load(f)
        
        assert 'fold_metrics' in metrics
        assert len(metrics['fold_metrics']) == 3
    
    def test_metrics_contains_best_fold_info(self, tmp_path, sample_labeled_file):
        """Verify metrics identify the best fold."""
        output_dir = tmp_path / "models"
        
        from unittest.mock import patch
        with patch('sys.argv', [
            'train.py',
            '--input', sample_labeled_file,
            '--output', str(output_dir),
            '--n-splits', '3'
        ]):
            main()
        
        metrics_path = output_dir / 'metrics.json'
        with open(metrics_path) as f:
            metrics = json.load(f)
        
        assert 'best_fold' in metrics
        assert isinstance(metrics['best_fold'], int)
        assert 1 <= metrics['best_fold'] <= 3


class TestFeatureNamesPreserved:
    """Test that feature names are preserved in the model."""
    
    def test_feature_names_preserved(self, tmp_path, sample_labeled_file):
        """Verify model has feature names."""
        output_dir = tmp_path / "models"
        
        from unittest.mock import patch
        with patch('sys.argv', [
            'train.py',
            '--input', sample_labeled_file,
            '--output', str(output_dir),
            '--n-splits', '3'
        ]):
            main()
        
        model_path = output_dir / 'model.joblib'
        model = joblib.load(model_path)
        
        assert hasattr(model, 'feature_name_'), "Model should have feature_name_ attribute"
        assert model.feature_name_ is not None
        assert len(model.feature_name_) == len(FEATURE_COLUMNS)
    
    def test_feature_names_in_metrics(self, tmp_path, sample_labeled_file):
        """Verify feature names are saved in metrics."""
        output_dir = tmp_path / "models"
        
        from unittest.mock import patch
        with patch('sys.argv', [
            'train.py',
            '--input', sample_labeled_file,
            '--output', str(output_dir),
            '--n-splits', '3'
        ]):
            main()
        
        metrics_path = output_dir / 'metrics.json'
        with open(metrics_path) as f:
            metrics = json.load(f)
        
        assert 'feature_names' in metrics
        assert metrics['feature_names'] == FEATURE_COLUMNS
        assert metrics['n_features'] == len(FEATURE_COLUMNS)


class TestDataLoading:
    """Test data loading functionality."""
    
    def test_load_labeled_data(self, sample_labeled_file, sample_labeled_data):
        """Test loading labeled data from parquet."""
        loaded_data = load_labeled_data(sample_labeled_file)
        
        assert len(loaded_data) == len(sample_labeled_data)
        assert 'label' in loaded_data.columns
        assert all(col in loaded_data.columns for col in FEATURE_COLUMNS)
    
    def test_load_labeled_data_missing_file(self, tmp_path):
        """Test handling of missing file."""
        missing_file = tmp_path / "nonexistent.parquet"
        
        with pytest.raises(FileNotFoundError):
            load_labeled_data(str(missing_file))
    
    def test_load_labeled_data_missing_features(self, tmp_path):
        """Test handling of missing feature columns."""
        df = pd.DataFrame({
            'returns': [0.01, 0.02],
            'label': ['UP', 'DOWN']
        })
        file_path = tmp_path / "incomplete.parquet"
        df.to_parquet(file_path, index=False)
        
        with pytest.raises(ValueError) as exc_info:
            load_labeled_data(str(file_path))
        assert "Missing feature columns" in str(exc_info.value)
    
    def test_load_labeled_data_missing_label(self, tmp_path):
        """Test handling of missing label column."""
        df = pd.DataFrame({
            col: [0.0] for col in FEATURE_COLUMNS
        })
        file_path = tmp_path / "no_label.parquet"
        df.to_parquet(file_path, index=False)
        
        with pytest.raises(ValueError) as exc_info:
            load_labeled_data(str(file_path))
        assert "Missing 'label' column" in str(exc_info.value)


class TestWalkForwardSplits:
    """Test walk-forward validation splits."""
    
    def test_create_walk_forward_splits(self, sample_labeled_data):
        """Verify splits are created correctly."""
        splits = create_walk_forward_splits(sample_labeled_data, n_splits=5)
        
        assert len(splits) == 5
        
        for train_idx, test_idx in splits:
            assert len(train_idx) > 0
            assert len(test_idx) > 0
            assert max(train_idx) < min(test_idx)


class TestModelTraining:
    """Test model training functionality."""
    
    def test_train_lightgbm_model(self, sample_labeled_data):
        """Test training a LightGBM model."""
        X = sample_labeled_data[FEATURE_COLUMNS][:50]
        y = np.array([0, 1, 2] * 16 + [0, 1])[:50]
        
        model = train_lightgbm_model(X, y, FEATURE_COLUMNS)
        
        assert model is not None
        assert hasattr(model, 'predict')
    
    def test_train_lightgbm_model_multiclass(self, sample_labeled_data):
        """Test that model is trained for multiclass classification."""
        X = sample_labeled_data[FEATURE_COLUMNS][:50]
        y = np.array([0, 1, 2] * 16 + [0, 1])[:50]
        
        model = train_lightgbm_model(X, y, FEATURE_COLUMNS)
        
        assert model.objective_ == 'multiclass'


class TestModelEvaluation:
    """Test model evaluation functionality."""
    
    def test_evaluate_model(self, sample_labeled_data):
        """Test model evaluation."""
        X = sample_labeled_data[FEATURE_COLUMNS][:50]
        y = np.array([0, 1, 2] * 16 + [0, 1])[:50]
        
        model = train_lightgbm_model(X, y, FEATURE_COLUMNS)
        
        X_test = sample_labeled_data[FEATURE_COLUMNS][50:100]
        y_test = np.array([0, 1, 2] * 16 + [0, 1])
        
        metrics = evaluate_model(model, X_test, y_test)
        
        assert 'accuracy' in metrics
        assert 'precision_macro' in metrics
        assert 'recall_macro' in metrics
        assert 'f1_macro' in metrics
        assert 'confusion_matrix' in metrics
    
    def test_evaluate_model_per_class_metrics(self, sample_labeled_data):
        """Test that per-class metrics are calculated."""
        X = sample_labeled_data[FEATURE_COLUMNS][:50]
        y = np.array([0, 1, 2] * 16 + [0, 1])[:50]
        
        model = train_lightgbm_model(X, y, FEATURE_COLUMNS)
        
        X_test = sample_labeled_data[FEATURE_COLUMNS][50:100]
        y_test = np.array([0, 1, 2] * 16 + [0, 1])
        
        metrics = evaluate_model(model, X_test, y_test)
        
        for label in LABELS:
            assert f'precision_{label.lower()}' in metrics
            assert f'recall_{label.lower()}' in metrics
            assert f'f1_{label.lower()}' in metrics


class TestEndToEnd:
    """End-to-end tests."""
    
    def test_full_training_pipeline(self, tmp_path, sample_labeled_file):
        """Test complete training pipeline."""
        output_dir = tmp_path / "models"
        
        from unittest.mock import patch
        with patch('sys.argv', [
            'train.py',
            '--input', sample_labeled_file,
            '--output', str(output_dir),
            '--n-splits', '2'
        ]):
            exit_code = main()
        
        assert exit_code == 0
        
        model_path = output_dir / 'model.joblib'
        metrics_path = output_dir / 'metrics.json'
        
        assert model_path.exists()
        assert metrics_path.exists()
        
        model = joblib.load(model_path)
        assert hasattr(model, 'predict')
        
        with open(metrics_path) as f:
            metrics = json.load(f)
        
        assert 'mean_metrics' in metrics
        assert 'fold_metrics' in metrics
    
    def test_cli_missing_input_file(self, tmp_path):
        """Test CLI with non-existent input file."""
        missing_file = tmp_path / "nonexistent.parquet"
        output_dir = tmp_path / "models"
        
        from unittest.mock import patch
        with patch('sys.argv', [
            'train.py',
            '--input', str(missing_file),
            '--output', str(output_dir)
        ]):
            exit_code = main()
        
        assert exit_code != 0
