# LOKASI: api/ml/index.py - Vercel Python Serverless Function
import sys
import os

# Pastikan direktori api/ml ada di path agar import relatif berfungsi di Vercel
sys.path.insert(0, os.path.dirname(__file__))

from flask import Flask, request, jsonify
from flask_cors import CORS
from _model import train_and_predict

app = Flask(__name__)
CORS(app)

@app.route('/api/ml/predict', methods=['POST'])
def predict():
    data = request.get_json()

    if not data or 'product_name' not in data or 'outlet_id' not in data:
        return jsonify({"error": "product_name dan outlet_id dibutuhkan"}), 400

    product_name = data['product_name']
    outlet_id = data['outlet_id']
    periods = data.get('periods', 7)

    try:
        prediction = train_and_predict(product_name, outlet_id, periods)

        if prediction is None:
            return jsonify({"message": "Data historis tidak cukup untuk membuat prediksi."}), 200

        return jsonify(prediction)
    except Exception as e:
        return jsonify({"error": f"Terjadi kesalahan: {e}"}), 500

# Vercel Python runtime menggunakan `app` sebagai WSGI handler
# Tidak perlu handler() terpisah - Vercel otomatis detect Flask `app`
