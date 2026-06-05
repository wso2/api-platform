#!/usr/bin/env python3
"""
WSO2 API Platform Mock Server
Runs on http://localhost:8080 by default. Set HTTPS env to true to enable HTTPS.
"""

from flask import Flask, request, jsonify
from flask_cors import CORS
from datetime import datetime
import uuid
import ssl
import os

USE_HTTPS = str(os.getenv('HTTPS', '')).lower() in ('1', 'true', 'yes', 'on')

app = Flask(__name__)

CORS_ORIGIN = os.getenv('CORS_ORIGIN', '*')
CORS(app, resources={r"/*": {"origins": CORS_ORIGIN}}, supports_credentials=True)

# Mock data storage
organizations = {}
projects = {}
apis = {}
gateways = {}
tokens = {}
devportals = {}
llm_providers = {}
llm_templates = {}
llm_proxies = {}


def generate_uuid():
    """Generate a UUID string"""
    return str(uuid.uuid4())


def get_current_timestamp():
    """Get current timestamp in ISO format"""
    return datetime.utcnow().strftime('%Y-%m-%dT%H:%M:%SZ')


def check_auth_header():
    """
    Check for Bearer token in Authorization header.
    Does NOT validate the token - just checks it exists.
    Returns True if header is present, False otherwise.
    """
    auth_header = request.headers.get('Authorization')
    if not auth_header:
        return False
    
    # Check if it starts with 'Bearer '
    if not auth_header.startswith('Bearer '):
        return False
    
    return True


def require_auth():
    """
    Decorator-like function to check auth.
    Returns error response if auth fails, None if auth passes.
    Auth is enforced only when USE_HTTPS is True.
    """
    if not USE_HTTPS:
        return None

    if not check_auth_header():
        return jsonify({
            "code": 401,
            "message": "Unauthorized",
            "description": "Authorization header with Bearer token is required"
        }), 401
    return None


# ============== LLM Provider Endpoints ==============

@app.route('/llm-providers', methods=['POST', 'OPTIONS'])
def create_llm_provider():
    """Create an LLM provider"""
    if request.method == 'OPTIONS':
        return '', 204
    
    auth_error = require_auth()
    if auth_error:
        return auth_error
    
    data = request.json
    provider_id = generate_uuid()
    
    provider = {
        "id": provider_id,
        "name": data.get("name", "OpenAI Provider"),
        "type": data.get("type", "openai"),
        "endpoint": data.get("endpoint"),
        "createdAt": get_current_timestamp(),
        "updatedAt": get_current_timestamp()
    }
    
    llm_providers[provider_id] = provider
    return jsonify(provider), 201


@app.route('/llm-providers', methods=['GET', 'OPTIONS'])
def list_llm_providers():
    """List all LLM providers"""
    if request.method == 'OPTIONS':
        return '', 204
    
    auth_error = require_auth()
    if auth_error:
        return auth_error
    
    if not llm_providers:
        mock_providers = [
            {
              "id": "openai",
              "name": "OpenAI",
              "models": 9,
              "lastUpdated": "2024-09-10T09:45:00Z",
              "status": "Active",
              "vhost": "api.openai.com",
              "version": "1.0"
            },
            {
              "id": "anthropic",
              "name": "Anthropic",
              "models": 5,
              "lastUpdated": "2024-09-12T14:30:00Z",
              "status": "Active",
              "vhost": "api.anthropic.com",
              "version": "1.1"
            },
            {
              "id": "azure-openai",
              "name": "Azure OpenAI",
              "models": 7,
              "lastUpdated": "2024-09-08T11:20:00Z",
              "status": "Degraded",
              "vhost": "openai.azure.com",
              "version": "2024-05-01"
            },
            {
              "id": "aws-bedrock",
              "name": "AWS Bedrock",
              "models": 6,
              "lastUpdated": "2024-09-11T17:10:00Z",
              "status": "Active",
              "vhost": "bedrock.fusion.com",
              "version": "1.0"
            },
            {
              "id": "google-vertex",
              "name": "Google Vertex",
              "models": 4,
              "lastUpdated": "2024-09-09T08:05:00Z",
              "status": "Active",
              "vhost": "vertexai.googleapis.com",
              "version": "v1"
            },
            {
              "id": "cohere",
              "name": "Cohere",
              "models": 3,
              "lastUpdated": "2024-09-07T18:50:00Z",
              "status": "Paused",
              "vhost": "api.cohere.ai",
              "version": "2024-06"
            }
        ]
        return jsonify(mock_providers), 200
    
    provider_list = list(llm_providers.values())
    return jsonify(provider_list), 200


@app.route('/llm-providers/<provider_id>', methods=['GET', 'OPTIONS'])
def get_llm_provider(provider_id):
    """Get LLM provider by ID"""
    if request.method == 'OPTIONS':
        return '', 204
    
    auth_error = require_auth()
    if auth_error:
        return auth_error
    
    if provider_id in llm_providers:
        return jsonify(llm_providers[provider_id]), 200
    
    # Return mock provider
    provider = {
        "id": provider_id,
        "name": "OpenAI Provider",
        "type": "openai",
        "endpoint": "https://api.openai.com/v1",
        "createdAt": "2025-01-15T10:00:00Z",
        "updatedAt": "2025-01-28T10:00:00Z"
    }
    return jsonify(provider), 200


@app.route('/llm-providers/<provider_id>', methods=['PUT', 'OPTIONS'])
def update_llm_provider(provider_id):
    """Update LLM provider"""
    if request.method == 'OPTIONS':
        return '', 204
    
    auth_error = require_auth()
    if auth_error:
        return auth_error
    
    data = request.json
    
    if provider_id in llm_providers:
        llm_providers[provider_id].update({
            "name": data.get("name", llm_providers[provider_id].get("name")),
            "endpoint": data.get("endpoint", llm_providers[provider_id].get("endpoint")),
            "updatedAt": get_current_timestamp()
        })
        return jsonify(llm_providers[provider_id]), 200
    
    # Return updated mock provider
    provider = {
        "id": provider_id,
        "name": data.get("name", "Updated Provider"),
        "type": "openai",
        "endpoint": data.get("endpoint", "https://api.example.com"),
        "updatedAt": get_current_timestamp()
    }
    return jsonify(provider), 200


@app.route('/llm-providers/<provider_id>', methods=['DELETE', 'OPTIONS'])
def delete_llm_provider(provider_id):
    """Delete LLM provider"""
    if request.method == 'OPTIONS':
        return '', 204
    
    auth_error = require_auth()
    if auth_error:
        return auth_error
    
    if provider_id in llm_providers:
        del llm_providers[provider_id]
    return '', 204

# ============== Error Handlers ==============

@app.errorhandler(400)
def bad_request(error):
    return jsonify({
        "code": 400,
        "message": "Bad Request",
        "description": "Invalid request or validation error"
    }), 400


@app.errorhandler(401)
def unauthorized(error):
    return jsonify({
        "code": 401,
        "message": "Unauthorized",
        "description": "Authorization header is required or token is invalid"
    }), 401


@app.errorhandler(404)
def not_found(error):
    return jsonify({
        "code": 404,
        "message": "Not Found",
        "description": "The specified resource does not exist"
    }), 404


@app.errorhandler(409)
def conflict(error):
    return jsonify({
        "code": 409,
        "message": "Conflict",
        "description": "Specified resource already exists"
    }), 409


@app.errorhandler(500)
def internal_error(error):
    return jsonify({
        "code": 500,
        "message": "Internal Server Error",
        "description": "The server encountered an internal error"
    }), 500


if __name__ == '__main__':
    protocol = 'https' if USE_HTTPS else 'http'
    print("=" * 60)
    print(f"WSO2 API Platform Mock Server ({protocol.upper()})")
    print("=" * 60)
    print(f"Server running on: {protocol}://localhost:8080")
    print("API base path: /api/v1")
    print("LLM Providers: /llm-providers (no /api/v1 prefix)")
    print("=" * 60)
    print("\nAuthentication:")
    if USE_HTTPS:
        print("  - All endpoints (except /health) require Bearer token")
        print("  - Example: Authorization: Bearer <any-token>")
        print("  - Tokens are NOT validated - any token will work")
    else:
        print("  - Authentication is NOT enforced in HTTP mode")
    print("=" * 60)
    print("\nCORS:")
    print("  - ALL ORIGINS ALLOWED (*)")
    print("  - ⚠️  WARNING: This is permissive and should only be")
    print("  - used for development/testing purposes")
    print("=" * 60)
    print("\nAvailable endpoints:")
    print("  - Health: GET /api/v1/health (no auth required)")
    print("  - Organizations: POST, GET /api/v1/organizations")
    print("  - Projects: POST, GET, PUT, DELETE /api/v1/projects")
    print("  - APIs: POST, GET, PUT, DELETE /api/v1/apis")
    print("  - Gateways: POST, GET, PUT, DELETE /api/v1/gateways")
    print("  - DevPortals: POST, GET, PUT, DELETE /api/v1/devportals")
    print("  - LLM Providers: POST, GET, PUT, DELETE /llm-providers")
    print("  - LLM Templates: POST, GET, PUT, DELETE /api/v1/llm-templates")
    print("  - LLM Proxies: POST, GET, PUT, DELETE /api/v1/llm-proxies")
    print("=" * 60)
    if USE_HTTPS:
        print("\nSSL Certificate:")
        print("  - Using adhoc (self-signed) certificate")
        print("  - Browsers will show security warning - this is normal")
        print("  - Visit https://localhost:8080/api/v1/health to trust cert")
        print("  - For curl: use --insecure or -k flag")
    else:
        print("\nSSL Certificate:")
        print("  - Not using SSL in HTTP mode")
    print("=" * 60)
    print("\nSetup:")
    print("  pip install flask flask-cors pyopenssl")
    print("=" * 60)

    # Run server with or without adhoc SSL context depending on USE_HTTPS
    if USE_HTTPS:
        app.run(host='0.0.0.0', port=8080, debug=True, ssl_context='adhoc')
    else:
        app.run(host='0.0.0.0', port=8080, debug=True)