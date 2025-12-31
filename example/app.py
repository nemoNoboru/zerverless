#!/usr/bin/env python3
"""
Flask app example for Zerverless GitOps deployment.
This app handles HTTP requests and returns JSON responses.
"""

import json
import sys
from flask import Flask, request, jsonify

app = Flask(__name__)

def handle(req):
    """
    Main handler function for Zerverless.
    Receives request dict and returns response dict.
    """
    method = req.get('method', 'GET')
    path = req.get('path', '/')
    query = req.get('query', {})
    headers = req.get('headers', {})
    body = req.get('body', '')
    
    # Parse JSON body if present
    try:
        if body:
            body_data = json.loads(body)
        else:
            body_data = {}
    except:
        body_data = {}
    
    # Route handling
    if path == '/' or path == '/hello':
        return {
            'status': 200,
            'headers': {'Content-Type': 'application/json'},
            'body': json.dumps({
                'message': 'Hello from Flask!',
                'path': path,
                'method': method
            })
        }
    
    elif path == '/api/users':
        if method == 'GET':
            return {
                'status': 200,
                'headers': {'Content-Type': 'application/json'},
                'body': json.dumps({
                    'users': [
                        {'id': 1, 'name': 'Alice'},
                        {'id': 2, 'name': 'Bob'}
                    ]
                })
            }
        elif method == 'POST':
            return {
                'status': 201,
                'headers': {'Content-Type': 'application/json'},
                'body': json.dumps({
                    'message': 'User created',
                    'data': body_data
                })
            }
    
    elif path.startswith('/api/users/'):
        user_id = path.split('/')[-1]
        return {
            'status': 200,
            'headers': {'Content-Type': 'application/json'},
            'body': json.dumps({
                'id': user_id,
                'name': f'User {user_id}',
                'path': path
            })
        }
    
    elif path == '/api/health':
        return {
            'status': 200,
            'headers': {'Content-Type': 'application/json'},
            'body': json.dumps({'status': 'healthy', 'service': 'flask-app'})
        }
    
    else:
        return {
            'status': 404,
            'headers': {'Content-Type': 'application/json'},
            'body': json.dumps({'error': 'Not found', 'path': path})
        }

if __name__ == '__main__':
    # For local development
    app.run(host='0.0.0.0', port=5000, debug=True)
else:
    # For Zerverless: read from stdin, write to stdout
    try:
        input_data = sys.stdin.read()
        req = json.loads(input_data) if input_data else {}
        result = handle(req)
        print(json.dumps(result))
    except Exception as e:
        error_response = {
            'status': 500,
            'headers': {'Content-Type': 'application/json'},
            'body': json.dumps({'error': str(e)})
        }
        print(json.dumps(error_response))

