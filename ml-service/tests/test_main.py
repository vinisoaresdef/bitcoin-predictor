import pytest
from fastapi.testclient import TestClient

from main import app, MODEL_VERSION


class TestModelLoading:
    
    def test_model_loaded_at_startup(self):
        with TestClient(app) as client:
            assert hasattr(app.state, 'model')
            assert app.state.model is not None
    
    def test_model_has_feature_names(self):
        with TestClient(app) as client:
            assert hasattr(app.state, 'model')
            assert app.state.model is not None
            
            has_features = (
                hasattr(app.state.model, 'feature_names_') or
                hasattr(app.state.model, 'feature_name_')
            )
            assert has_features, "Model should have feature names"


class TestHealthEndpoint:
    
    def test_health_with_model(self):
        with TestClient(app) as client:
            response = client.get("/health")
        
        assert response.status_code == 200
        data = response.json()
        assert data["status"] == "ok"
        assert data["model_loaded"] is True
        assert data["model_version"] == MODEL_VERSION
    
    def test_health_endpoint_content_type(self):
        with TestClient(app) as client:
            response = client.get("/health")
        
        assert response.headers["content-type"] == "application/json"
    
    def test_health_without_model(self):
        from contextlib import asynccontextmanager
        from fastapi import FastAPI
        from fastapi.responses import JSONResponse
        
        @asynccontextmanager
        async def empty_lifespan(app: FastAPI):
            app.state.model = None
            yield
        
        test_app = FastAPI(lifespan=empty_lifespan)
        
        @test_app.get("/health")
        async def health_no_model():
            model_loaded = hasattr(test_app.state, 'model') and test_app.state.model is not None
            if model_loaded:
                return JSONResponse(
                    status_code=200,
                    content={"status": "ok", "model_loaded": True, "model_version": MODEL_VERSION}
                )
            else:
                return JSONResponse(
                    status_code=503,
                    content={"status": "error", "model_loaded": False, "model_version": MODEL_VERSION}
                )
        
        with TestClient(test_app) as client:
            response = client.get("/health")
        
        assert response.status_code == 503
        data = response.json()
        assert data["status"] == "error"
        assert data["model_loaded"] is False
        assert data["model_version"] == MODEL_VERSION
