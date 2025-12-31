# Flask Example App for Zerverless GitOps

This is an example Flask application that can be deployed via Zerverless GitOps.

## Structure

```
example/
├── app.py              # Flask application
├── Dockerfile          # Docker container definition
├── requirements.txt    # Python dependencies
├── zerverless.yaml     # GitOps deployment manifest
└── static/            # Static files
    ├── index.html
    ├── css/
    │   └── style.css
    └── js/
        └── app.js
```

## Features

- **Flask API** with multiple endpoints:
  - `GET /flask-example/hello` - Simple hello endpoint
  - `GET /flask-example/users` - List users
  - `POST /flask-example/users` - Create user
  - `GET /flask-example/users/:id` - Get user by ID
  - `GET /flask-example/health` - Health check

- **Static files** served from `/static/example/`
- **Docker deployment** via GitOps

## Deployment

### 1. Push to Git Repository

```bash
git init
git add .
git commit -m "Initial Flask app"
git remote add origin https://github.com/yourusername/zerverless-example.git
git push -u origin main
```

### 2. Register with Zerverless

```bash
curl -X POST http://localhost:8000/api/gitops/applications \
  -H "Content-Type: application/json" \
  -d '{
    "name": "flask-example",
    "namespace": "example",
    "source": {
      "repoURL": "https://github.com/yourusername/zerverless-example.git",
      "branch": "main"
    }
  }'
```

### 3. Access the App

- **API**: `http://localhost:8000/example/flask-example/hello`
- **Static files**: `http://localhost:8000/static/example/index.html`

## Local Development

```bash
# Install dependencies
pip install -r requirements.txt

# Run Flask app
python app.py
```

## Testing

```bash
# Test hello endpoint
curl http://localhost:8000/example/flask-example/hello

# Test users endpoint
curl http://localhost:8000/example/flask-example/users

# Test health endpoint
curl http://localhost:8000/example/flask-example/health
```

## How It Works

1. **GitOps Sync**: Zerverless polls the Git repository every 5 minutes
2. **Docker Build**: When changes are detected, it builds the Docker image from the Dockerfile
3. **Deployment**: The Flask app is deployed as a function at `/example/flask-example/*`
4. **Static Files**: Files in `static/` are synced to storage and served at `/static/example/*`

## Customization

- Update `zerverless.yaml` to change the namespace or paths
- Modify `app.py` to add new endpoints
- Add more static files to the `static/` directory
- Update `requirements.txt` to add Python dependencies

# small-zerverless-test-flask-docker
