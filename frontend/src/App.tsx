import React, { useState, type ChangeEvent } from "react";
import Papa from "papaparse";

interface StatusMessage {
  type: "success" | "error";
  text: string;
}

export default function App() {
  const [message, setMessage] = useState<string>("");
  const [rawNumbers, setRawNumbers] = useState<string>("");
  const [imageBase64, setImageBase64] = useState<string>("");
  const [loading, setLoading] = useState<boolean>(false);
  const [status, setStatus] = useState<StatusMessage | null>(null);

  // 1. Handler Import File CSV
  const handleCsvUpload = (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    Papa.parse(file, {
      header: true,
      skipEmptyLines: true,
      complete: (results) => {
        const numbers: string[] = [];

        // Parsing setiap baris CSV untuk mengambil kolom yang berisi nomor HP
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        results.data.forEach((row: any) => {
          // Mencari kolom bernama 'nomor', 'phone', 'whatsapp', 'hp', atau mengambil kolom pertama jika nama beda
          const phoneValue =
            row.nomor ||
            row.Nomor ||
            row.phone ||
            row.Phone ||
            row.whatsapp ||
            row.hp ||
            Object.values(row)[0];

          if (phoneValue && typeof phoneValue === "string") {
            numbers.push(phoneValue.trim());
          }
        });

        if (numbers.length > 0) {
          // Gabungkan nomor-nomor baru dari CSV ke dalam textarea (dipisah baris baru)
          setRawNumbers((prev) => {
            const existing = prev ? prev.trim() + "\n" : "";
            return existing + numbers.join("\n");
          });
          setStatus({
            type: "success",
            text: `✅ Berhasil mengimpor ${numbers.length} nomor dari file CSV.`,
          });
        } else {
          setStatus({
            type: "error",
            text: "Format CSV tidak valid atau kolom nomor tidak ditemukan.",
          });
        }
      },
      error: (error) => {
        console.error("Error parsing CSV:", error);
        setStatus({ type: "error", text: "Gagal membaca file CSV." });
      },
    });

    // Reset input file agar file yang sama bisa diunggah ulang jika dibutuhkan
    e.target.value = "";
  };

  // 2. Handler Import File Gambar (Base64)
  const handleFileChange = (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      const reader = new FileReader();
      reader.onloadend = () => {
        if (typeof reader.result === "string") {
          setImageBase64(reader.result);
        }
      };
      reader.readAsDataURL(file);
    } else {
      setImageBase64("");
    }
  };

  // 3. Handler Submit Form Blast
  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setStatus(null);

    const numbersArray = rawNumbers
      .split(/[\n,]+/)
      .map((num) => num.trim())
      .filter((num) => num.length > 0);

    if (numbersArray.length === 0 || !message) {
      setStatus({
        type: "error",
        text: "Nomor kustomer dan pesan tidak boleh kosong!",
      });
      setLoading(false);
      return;
    }

    try {
      const response = await fetch("http://localhost:3000/api/blast", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          numbers: numbersArray,
          message: message,
          image_data: imageBase64,
        }),
      });

      const data = await response.json();

      if (response.ok) {
        setStatus({
          type: "success",
          text: `🚀 ${data.message} Total target: ${data.target} nomor.`,
        });
        setMessage("");
        setRawNumbers("");
        setImageBase64("");

        // Reset input gambar di DOM
        const imageInput = document.getElementById(
          "image-file",
        ) as HTMLInputElement;
        if (imageInput) imageInput.value = "";
      } else {
        setStatus({
          type: "error",
          text: data.error || "Gagal menjalankan blast.",
        });
      }
    } catch (error) {
      console.error(error);
      setStatus({
        type: "error",
        text: "Gagal terhubung ke server backend Go Fiber.",
      });
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
          Vite + Go Fiber Engine • Import CSV & Support Foto
        </p>
      </div>

      <div className="mt-8 sm:mx-auto sm:w-full sm:max-w-xl">
        <div className="bg-white py-8 px-6 shadow-md rounded-xl border border-slate-200">
          <form className="space-y-6" onSubmit={handleSubmit}>
            {/* Area Input File CSV */}
            <div className="bg-slate-50 p-4 rounded-lg border border-dashed border-slate-300">
              <label
                htmlFor="csv-file"
                className="block text-sm font-semibold text-slate-700 mb-1"
              >
                📂 Import Kontak dari File CSV
              </label>
              <input
                id="csv-file"
                type="file"
                accept=".csv"
                onChange={handleCsvUpload}
                disabled={loading}
                className="block w-full text-xs text-slate-500 file:mr-4 file:py-1.5 file:px-3 file:rounded-md file:border-0 file:text-xs file:font-semibold file:bg-slate-200 file:text-slate-700 hover:file:bg-slate-300 cursor-pointer"
              />
              <p className="mt-1.5 text-[11px] text-slate-400">
                Gunakan file CSV dengan nama kolom:{" "}
                <code className="bg-slate-200 px-1 rounded">nomor</code>,{" "}
                <code className="bg-slate-200 px-1 rounded">phone</code>, atau{" "}
                <code className="bg-slate-200 px-1 rounded">hp</code>.
              </p>
            </div>

            {/* Area Textarea Nomor Customer */}
            <div>
              <div className="flex justify-between items-center mb-1">
                <div className="flex items-center gap-2">
                  <label
                    htmlFor="numbers"
                    className="block text-sm font-semibold text-slate-700"
                  >
                    Daftar Nomor Tujuan
                  </label>
                  {rawNumbers && (
                    <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-emerald-100 text-emerald-800">
                      {
                        rawNumbers
                          .split(/[\n,]+/)
                          .map((num) => num.trim())
                          .filter((num) => num.length > 0).length
                      }{" "}
                      nomor
                    </span>
                  )}
                </div>
                {rawNumbers && (
                  <button
                    type="button"
                    onClick={() => setRawNumbers("")}
                    className="text-xs text-rose-600 hover:underline"
                  >
                    Bersihkan Daftar
                  </button>
                )}
              </div>
              <textarea
                id="numbers"
                rows={5}
                className="shadow-sm focus:ring-emerald-500 focus:border-emerald-500 block w-full sm:text-sm border border-slate-300 rounded-lg p-3 font-mono text-slate-800"
                placeholder="Hasil import CSV atau ketik manual di sini (1 nomor per baris)..."
                value={rawNumbers}
                onChange={(e) => setRawNumbers(e.target.value)}
                disabled={loading}
              />
            </div>

            {/* Input Lampiran Foto */}
            <div>
              <label
                htmlFor="image-file"
                className="block text-sm font-semibold text-slate-700 mb-1"
              >
                Lampirkan Foto (Opsional)
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
                  <p className="text-xs text-emerald-600 font-medium">
                    ✓ Gambar berhasil dimuat.
                  </p>
                </div>
              )}
            </div>

            {/* Input Pesan / Caption */}
            <div>
              <label
                htmlFor="message"
                className="block text-sm font-semibold text-slate-700 mb-1"
              >
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

            {/* Status Alert */}
            {status && (
              <div
                className={`p-4 rounded-lg text-sm border ${
                  status.type === "success"
                    ? "bg-emerald-50 text-emerald-800 border-emerald-200"
                    : "bg-rose-50 text-rose-800 border-rose-200"
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
                    ? "bg-slate-400 cursor-not-allowed"
                    : "bg-emerald-600 hover:bg-emerald-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-emerald-500"
                }`}
              >
                {loading ? "Mengolah Data..." : "🚀 Mulai Kirim Massal"}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  );
}
