import React, { useState, type ChangeEvent } from 'react';

interface StatusMessage {
  type: 'success' | 'error';
  text: string;
}

export default function App() {
  const [message, setMessage] = useState<string>('');
  const [rawNumbers, setRawNumbers] = useState<string>('');
  const [imageBase64, setImageBase64] = useState<string>('');
  const [loading, setLoading] = useState<boolean>(false);
  const [status, setStatus] = useState<StatusMessage | null>(null);

  // Fungsi mengubah file gambar fisik menjadi String Base64
  const handleFileChange = (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      const reader = new FileReader();
      reader.onloadend = () => {
        if (typeof reader.result === 'string') {
          setImageBase64(reader.result);
        }
      };
      reader.readAsDataURL(file);
    } else {
      setImageBase64('');
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setStatus(null);

    const numbersArray = rawNumbers
      .split(/[\n,]+/)
      .map((num) => num.trim())
      .filter((num) => num.length > 0);

    if (numbersArray.length === 0 || !message) {
      setStatus({ type: 'error', text: 'Nomor kustomer dan pesan tidak boleh kosong!' });
      setLoading(false);
      return;
    }

    try {
      const response = await fetch('http://localhost:3000/api/blast', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          numbers: numbersArray,
          message: message,
          image_data: imageBase64, // Kirim string Base64 gambar ke backend
        }),
      });

      const data = await response.json();

      if (response.ok) {
        setStatus({
          type: 'success',
          text: `🚀 ${data.message} Total target: ${data.target} nomor.`,
        });
        setMessage('');
        setRawNumbers('');
        setImageBase64('');
        // Reset input file di DOM secara manual
        const fileInput = document.getElementById('image-file') as HTMLInputElement;
        if (fileInput) fileInput.value = '';
      } else {
        setStatus({ type: 'error', text: data.error || 'Gagal menjalankan blast.' });
      }
    } catch (error) {
      console.error(error);
      setStatus({ type: 'error', text: 'Gagal terhubung ke server backend Go Fiber.' });
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-slate-50 flex flex-col justify-center py-12 px-4 sm:px-6 lg:px-8 font-sans">
      <div className="sm:mx-auto sm:w-full sm:max-w-md">
        <h2 className="text-center text-3xl font-black text-slate-800 tracking-tight">
          🟢 WA Blast Dashboard
        </h2>
        <p className="mt-2 text-center text-sm text-slate-500">
          Vite + Go Fiber Engine • Mendukung Kirim Teks & Foto
        </p>
      </div>

      <div className="mt-8 sm:mx-auto sm:w-full sm:max-w-xl">
        <div className="bg-white py-8 px-6 shadow-md rounded-xl border border-slate-200">
          <form className="space-y-6" onSubmit={handleSubmit}>
            
            {/* Input Kustomer */}
            <div>
              <label htmlFor="numbers" className="block text-sm font-semibold text-slate-700 mb-1">
                Daftar Nomor Tujuan (Satu nomor per baris / pisah koma)
              </label>
              <textarea
                id="numbers"
                rows={5}
                className="shadow-sm focus:ring-emerald-500 focus:border-emerald-500 block w-full sm:text-sm border border-slate-300 rounded-lg p-3 font-mono text-slate-800"
                placeholder="Contoh:&#10;081234567890&#10;089876543210"
                value={rawNumbers}
                onChange={(e) => setRawNumbers(e.target.value)}
                disabled={loading}
              />
            </div>

            {/* Input Lampiran Foto (Baru) */}
            <div>
              <label htmlFor="image-file" className="block text-sm font-semibold text-slate-700 mb-1">
                Lampirkan Foto (Opsional, format .jpg/.png)
              </label>
              <input
                id="image-file"
                type="file"
                accept="image/png, image/jpeg, image/jpg"
                onChange={handleFileChange}
                disabled={loading}
                className="block w-full text-sm text-slate-500 file:mr-4 file:py-2 file:px-4 file:rounded-full file:border-0 file:text-sm file:font-semibold file:bg-emerald-50 file:text-emerald-700 hover:file:bg-emerald-100 cursor-pointer"
              />
              {imageBase64 && (
                <div className="mt-2">
                  <p className="text-xs text-emerald-600 font-medium">✓ Gambar berhasil dimuat dan siap dikirim.</p>
                </div>
              )}
            </div>

            {/* Input Teks/Caption */}
            <div>
              <label htmlFor="message" className="block text-sm font-semibold text-slate-700 mb-1">
                Isi Pesan / Caption Foto
              </label>
              <textarea
                id="message"
                rows={4}
                className="shadow-sm focus:ring-emerald-500 focus:border-emerald-500 block w-full sm:text-sm border border-slate-300 rounded-lg p-3 text-slate-800"
                placeholder="Tulis pesan Anda di sini..."
                value={message}
                onChange={(e) => setMessage(e.target.value)}
                disabled={loading}
              />
            </div>

            {/* Notifikasi Status */}
            {status && (
              <div
                className={`p-4 rounded-lg text-sm border ${
                  status.type === 'success' 
                    ? 'bg-emerald-50 text-emerald-800 border-emerald-200' 
                    : 'bg-rose-50 text-rose-800 border-rose-200'
                }`}
              >
                {status.text}
              </div>
            )}

            {/* Tombol Aksi */}
            <div>
              <button
                type="submit"
                disabled={loading}
                className={`w-full flex justify-center py-3 px-4 border border-transparent rounded-lg shadow-sm text-sm font-bold text-white transition-colors ${
                  loading
                    ? 'bg-slate-400 cursor-not-allowed'
                    : 'bg-emerald-600 hover:bg-emerald-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-emerald-500'
                }`}
              >
                {loading ? 'Mengolah Data Media...' : '🚀 Mulai Kirim Massal'}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  );
}