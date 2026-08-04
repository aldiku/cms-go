# Spesifikasi Modul Product — Advertising Marketplace

Status: **draft teknis**, hasil rapikan dari catatan awal. Bagian [§9](#9-pertanyaan-terbuka--perlu-klarifikasi) berisi hal-hal yang masih ambigu di catatan asli dan perlu dikonfirmasi sebelum implementasi.

## Daftar Isi

1. [Ringkasan](#1-ringkasan)
2. [Model Data Inti](#2-model-data-inti)
3. [Struktur Kategori & Produk per Vertikal](#3-struktur-kategori--produk-per-vertikal)
4. [Audience / Targeting](#4-audience--targeting)
5. [Creative](#5-creative)
6. [Alur Registrasi Layanan](#6-alur-registrasi-layanan)
7. [DOOH — Data Existing](#7-dooh--data-existing)
8. [Sistem Harga Berjenjang (Reseller Pricing)](#8-sistem-harga-berjenjang-reseller-pricing)
9. [Pertanyaan Terbuka / Perlu Klarifikasi](#9-pertanyaan-terbuka--perlu-klarifikasi)

---

## 1. Ringkasan

Modul **Product** mengelola katalog layanan advertising yang dijual melalui marketplace: SMS/RCS Ads, WhatsApp Ads, Online Ads (Meta/Google/YouTube/TikTok/Programmatic), dan Outdoor Ads (DOOH). Selain katalog, modul ini mencakup:

- **Audience** — definisi target penerima/penonton iklan.
- **Creative** — aset iklan (gambar, video, copy) yang dipasangkan dengan campaign.
- **Registrasi layanan** — onboarding sender ID untuk SMS Broadcast dan WhatsApp Business API (WABA).
- **Harga berjenjang** — HPP/price per produk, dengan kemampuan admin dan reseller melakukan override harga secara bertingkat.

---

## 2. Model Data Inti

### 2.1 Category

Kategori tingkat atas (top-level), flat, tidak bertingkat. CRUD penuh.

| Field | Tipe | Keterangan |
|---|---|---|
| `id` | integer | Primary key |
| `nama` | string | Nama kategori (contoh: "SMS Advertising") |
| `deskripsi` | text | Deskripsi kategori |
| `gambar` | string | URL/path gambar kategori |

### 2.2 Product (Hierarkis)

Produk berada di bawah satu Category, dan **bersifat rekursif** — satu Product bisa punya Product anak (lihat pohon di [§3](#3-struktur-kategori--produk-per-vertikal), contoh: `SMS Advertising > SMS LBA > TELKOMSEL > Telkomsel Ads` = 4 level bersarang). CRUD penuh.

| Field | Tipe | Keterangan |
|---|---|---|
| `id` | integer | Primary key |
| `code` | string | Kode unik produk |
| `cat_id` | integer | FK ke `Category.id` |
| `parent_id` | integer, nullable | FK ke `Product.id` sendiri — mendukung nesting N-level. `null` = node tingkat pertama di bawah kategori |
| `nama` | string | Nama produk/node (contoh: "SMS LBA", "TELKOMSEL", "Telkomsel Ads") |
| `deskripsi` | text | Deskripsi produk |
| `thumbnail` | string | URL/path thumbnail |
| `base_price` / `hpp` | numeric | Harga Pokok Penjualan (cost) |
| `price` | numeric | Harga jual default |
| `status` | string/int | Status aktif/nonaktif |
| `list_order` | integer | Urutan tampil |

> Lihat [§9.1](#9-pertanyaan-terbuka--perlu-klarifikasi) — perlu konfirmasi apakah **setiap** level di pohon adalah row `Product` (dengan `parent_id`), atau ada level yang sebetulnya harus jadi entity terpisah (mis. "operator" sebagai entity, bukan node produk).

### 2.3 Product Variant

Unit yang benar-benar dijual/di-*checkout* di level daun (leaf) pohon Product. CRUD penuh.

| Field | Tipe | Keterangan |
|---|---|---|
| `id` | integer | Primary key |
| `product_id` | integer | FK ke `Product.id` (biasanya node daun) |
| `nama` | string | Label varian |
| `hpp` | numeric | Harga pokok varian ini |
| `price` | numeric | Harga jual default varian ini |
| `status` | string/int | Status aktif/nonaktif |
| `list_order` | integer | Urutan tampil |

> Field di atas adalah minimum yang bisa disimpulkan dari catatan asli (disebutkan varian punya `hpp` dan `price`, serta bisa punya *tiering*). Field lain seperti kode SKU atau satuan (mis. "per session") belum eksplisit — lihat [§9.2](#9-pertanyaan-terbuka--perlu-klarifikasi).

**Tiering harga per varian** (dipakai juga di WABA, [§6.2](#62-registrasi-whatsapp-business-api-waba)): satu varian bisa punya mode **fixed price** (satu angka) atau **tiering price** (beberapa titik harga berdasarkan volume, contoh sesi WABA: 10.000 / 20.000 / 50.000, plus opsi "custom session" yang perlu approval manual).

| Field (tabel `product_variant_tier`, usulan) | Tipe | Keterangan |
|---|---|---|
| `id` | integer | Primary key |
| `variant_id` | integer | FK ke `ProductVariant.id` |
| `label` | string | Label tier (contoh: "10.000 sesi") |
| `price` | numeric | Harga untuk tier ini |
| `is_custom` | boolean | `true` jika tier "custom" (harga dinegosiasikan, bukan dari daftar tetap) |

---

## 3. Struktur Kategori & Produk per Vertikal

Referensi isi pohon Category → Product (bersarang) untuk 4 kategori utama. Kedalaman tiap cabang **tidak seragam** (ada yang 2 level, ada yang 4 level) — konfirmasi dampaknya ke desain di [§9.1](#9-pertanyaan-terbuka--perlu-klarifikasi).

### 3.1 SMS Advertising

```
SMS Advertising
├─ SMS LBA (Location Based Ads)
│  ├─ TELKOMSEL → Telkomsel Ads
│  └─ IOH → Indosat Ads
├─ SMS Targetted
│  ├─ TELKOMSEL → Telkomsel Ads
│  └─ IOH → Indosat Ads
├─ SMS Broadcast
│  └─ Sender ID (dipilih dari sender ID yang sudah teregistrasi — lihat §6.1)
└─ RCS Ads
   └─ Telkomsel RCS
```

### 3.2 WhatsApp Advertising

```
WhatsApp Advertising
├─ WA LBA (Location Based Ads)
│  ├─ TELKOMSEL → Telkomsel Promo & Ads, Adsqoo
│  └─ IOH → Indosat Promo & Reward
├─ WA Targetted (location and interest)
│  ├─ TELKOMSEL → Telkomsel Promo & Ads, Adsqoo
│  └─ IOH → Indosat Promo & Reward
└─ WA Business API (WABA)
   └─ Sender ID (dipilih dari sender ID yang sudah teregistrasi — lihat §6.2)
```

### 3.3 Online Advertising

Tidak bersarang — 6 produk langsung di bawah kategori:

- Adxelerate Ads (Programmatic Ads)
- Meta Ads (Managed Service)
- Google Ads (Managed Service)
- YouTube Ads (Managed Service)
- TikTok Ads (Managed Service)
- Push Notification Ads

### 3.4 Outdoor Advertising (DOOH)

```
Outdoor Advertising
├─ DOOH VideoTron    → daftar DOOH dipilih (pagination), filter type = videotron
├─ DOOH Billboard    → daftar DOOH dipilih (pagination), filter type = billboard
└─ DOOH Mobile Truck → daftar DOOH dipilih (pagination), filter type = mobile-videotron
```

> Catatan asli menulis "type video tron" untuk ketiga cabang (VideoTron, Billboard, Mobile Truck). Nilai `type` di atas sudah dikoreksi berdasarkan data aktual di tabel `dooh_data` ([§7](#7-dooh--data-existing)), yang punya nilai `videotron`, `billboard`, dan `mobile-videotron`. **Perlu dikonfirmasi.**

---

## 4. Audience / Targeting

CRUD penuh. Merepresentasikan target penerima/penonton untuk satu campaign.

| Field | Tipe | Keterangan |
|---|---|---|
| `audience_id` | string | Format `AUD` + `YYMMDD` + random (contoh: `AUD260901HHGTGH`) — **aturan generate perlu dikonfirmasi**, lihat §9 |
| `type` | string | Tipe target, biasanya match dengan node produk daun (contoh: `sms-lba`) |
| `location` | string | Alamat/deskripsi lokasi (free text) |
| `latitude` | numeric | Titik tengah radius target |
| `longitude` | numeric | Titik tengah radius target |
| `radius` | integer | Radius target dalam meter |
| `prov_id` | integer | Referensi provinsi |
| `kab_id` | integer | Referensi kabupaten/kota |
| `kec_id` | integer | Referensi kecamatan |
| `kel_id` | integer, nullable | Referensi kelurahan |
| `whitelist` | array\<string\> | Daftar nomor HP yang di-whitelist manual |
| `arpu` | object | Filter ARPU: `{ "operand": ">", "value": 5000 }` |
| `gender` | array\<string\> | Contoh: `["all"]`, atau daftar gender spesifik |
| `religion` | array\<string\> | Contoh: `["all"]`, atau daftar agama spesifik |
| `min_age` | integer | Batas bawah usia |
| `max_age` | integer | Batas atas usia |
| `file_url` | string | URL CSV berisi daftar nomor HP — cara targeting alternatif selain filter demografis |
| `user_id` | integer | Pemilik/pembuat audience |
| `source` | string | `web` atau `api` |
| `sandbox` | integer | `0` = production, `1` = sandbox |

**Contoh payload** (dirapikan dari catatan asli — versi asli tidak valid sebagai JSON, ada key tanpa quote dan value yang bocor keluar string):

```json
{
  "audience_id": "AUD260901HHGTGH",
  "type": "sms-lba",
  "location": "Cihuni, Pagedangan, Tangerang Regency, Banten 10000, Indonesia",
  "latitude": -6.2727352,
  "longitude": 106.6361945,
  "radius": 1000,
  "prov_id": 33,
  "kab_id": 3322,
  "kec_id": 332201,
  "kel_id": null,
  "whitelist": ["08562376253", "073687234767"],
  "arpu": { "operand": ">", "value": 5000 },
  "gender": ["all"],
  "religion": ["all"],
  "min_age": 18,
  "max_age": 55,
  "file_url": "http://file.csv",
  "user_id": 1,
  "source": "web",
  "sandbox": 0
}
```

---

## 5. Creative

Aset iklan yang dipasangkan ke campaign. Field bersifat superset — tidak semua field terisi untuk setiap `type` creative (mis. `video_url` untuk creative video, `document_url` untuk WABA document, dst).

| Field | Tipe | Keterangan |
|---|---|---|
| `creative_id` | varchar | Primary key |
| `user_id` | bigint | Pemilik creative |
| `type` | varchar(20) | Tipe creative |
| `name` | varchar(100) | Nama creative |
| `media` | varchar, nullable | |
| `banner` | varchar, nullable | |
| `video_url` | varchar, nullable | |
| `image_url` | varchar, nullable | |
| `logo_url` | varchar, nullable | |
| `headline` | varchar, nullable | |
| `description` | text, nullable | |
| `primary_text` | text, nullable | |
| `call_to_action` | varchar, nullable | |
| `caption` | text, nullable | |
| `hashtag` | varchar, nullable | |
| `website_url` | varchar, nullable | |
| `fanspage_url` | varchar, nullable | |
| `youtube_url` | varchar, nullable | |
| `instagram_url` | varchar, nullable | |
| `tiktok_url` | varchar, nullable | |
| `file_name` | varchar, nullable | |
| `file_type` | varchar, nullable | |
| `document_url` | varchar, nullable | |
| `footer_type` | varchar, nullable | |
| `footer_cta` | text, nullable | |
| `source` | varchar, nullable, default `web` | |
| `sandbox` | integer, nullable | `0` = production, `1` = sandbox |
| `created_at` | timestamptz, default `now()` | |
| `updated_at` | timestamptz, default `now()` | |
| `deleted_at` | timestamptz, nullable | Soft delete |

---

## 6. Alur Registrasi Layanan

Sebelum sebuah sender ID bisa dipakai di campaign (lihat referensi "Sender ID" di §3.1 dan §3.2), perlu registrasi berikut.

### 6.1 Registrasi SMS Broadcast

| Field | Keterangan |
|---|---|
| Dokumen | Upload file `.zip` (form/template download disediakan sebagai link) |
| Nama sender | Nama sender ID yang didaftarkan |
| Token awal | Minimum **5.000** |

### 6.2 Registrasi WhatsApp Business API (WABA)

| Field | Keterangan |
|---|---|
| Dokumen | Upload file `.zip` (form/template download disediakan sebagai link) |
| Nama sender | Nama sender ID yang didaftarkan |
| Penanggung jawab | Nama, email, gender |
| Company | Nama perusahaan |
| Phone number | Nomor kontak |
| Website | URL website |
| Business Manager ID | ID Meta Business Manager |
| Link FB Page | URL halaman Facebook |
| Initial product | Tipe platform service: `required`, `utility`, `marketing`, atau `authentication` |

**Konfigurasi harga initial product** — pilih salah satu mode:
- **Fixed price** — satu harga tetap.
- **Tiering price** — beberapa titik harga per volume sesi, contoh: 10.000 / 20.000 / 50.000 sesi, plus opsi **custom session** (volume di luar daftar, harga dinegosiasikan).

---

## 7. DOOH — Data Existing

Tabel `dooh_data` **sudah ada** di database (data sudah terisi) — tugasnya adalah *clone struktur* dan bangun CRUD di atasnya, bukan membuat skema baru.

| Kolom | Tipe | Keterangan |
|---|---|---|
| `id` | integer | Primary key |
| `type` | varchar(250) | `videotron` / `mobile-videotron` / `billboard` (lihat data sample) |
| `user_id` | integer | Pemilik data |
| `name` | varchar(255) | |
| `latitude` | numeric(10,8), nullable | |
| `longitude` | numeric(11,8), nullable | |
| `geohash` | varchar(20), nullable | |
| `geo_location` | text, nullable | |
| `job_status` | varchar(50), nullable, default `SUCCESS` | |
| `job_status_code` | integer, nullable, default `2` | |
| `billboard_type` | varchar(100), nullable | |
| `display_name` | varchar(255) | |
| `country` | varchar(100), nullable, default `Indonesia` | |
| `province` | varchar(100), nullable | |
| `city` | varchar(250), nullable | |
| `district` | varchar(250), nullable | |
| `village` | varchar(250), nullable | |
| `board_facing` | varchar(100), nullable | |
| `num_of_panel` | integer, nullable, default `0` | |
| `size` | varchar(250), nullable | |
| `access` | varchar(100), nullable | |
| `image` | text, nullable | URL gambar |
| `reach` | bigint, nullable, default `0` | |
| `potential` | bigint, nullable, default `0` | |
| `user_access` | boolean, nullable, default `false` | |
| `for_planner` | boolean, nullable, default `false` | |
| `start_date` | timestamp, nullable | |
| `end_date` | timestamp, nullable | |
| `created_at` | timestamp, nullable, default `CURRENT_TIMESTAMP` | |
| `updated_at` | timestamp, nullable, default `CURRENT_TIMESTAMP` | |
| `display_name_type_city` | varchar(255), nullable | |
| `orientation` | varchar(250), nullable | |
| `resolution` | varchar(250), nullable | |
| `min_spot` | integer, nullable, default `0` | Kelipatan minimum pembelian spot |
| `hpp` | integer, nullable | |
| `price` | integer, nullable | |
| `info_tambahan` | text, nullable | Free text: durasi video, rotasi iklan, jadwal, dsb |
| `data_start_date` | varchar, nullable | |
| `data_end_date` | varchar, nullable | |
| `data_source` | varchar, nullable | |

**Contoh data** (1 row diringkas dari sample CSV asli — VideoTron):

| id | type | name | city | size | hpp | price | min_spot |
|---|---|---|---|---|---|---|---|
| 9 | videotron | LED. Jl. Underpass SCBD - SCBD LOT 9 | Kota Administrasi Jakarta Selatan | 10x5 | 28000 | 30000 | 180 |

Field `info_tambahan` pada row ini berisi: *"Durasi video: 15 detik, Rotasi Iklan: setiap 1.5 menit/iklan. Untuk pembelian spot di videotron ini harus sesuai kelipatan minimum spot. Jadwal penayangan secara random sesuai jam operasional LED Videotron. Batas waktu pemesanan 3 hari kerja."*

Dua contoh lain (Mobile Truck & Billboard) ada di data asli — polanya sama, tidak diulang di sini.

---

## 8. Sistem Harga Berjenjang (Reseller Pricing)

Ini bagian paling kompleks dari modul ini: **setiap level dalam hierarki reseller bisa override harga untuk level di bawahnya**, dan reseller bisa melihat margin keuntungannya sendiri di dashboard.

### 8.1 Aturan

1. Setiap `ProductVariant` (atau tier-nya) punya `hpp` dan `price` default yang diset admin — ini berlaku untuk **semua client** kecuali di-override.
2. Admin bisa set **harga custom per variant** untuk **user tertentu** (biasanya reseller) — override ini menggantikan harga default untuk user tersebut.
3. **Khusus role reseller**: reseller bisa set harga custom untuk client yang berada di bawahnya (relasi "di bawah siapa" ini sudah dimodelkan lewat `User.ReferralID` yang ada di modul auth — lihat [§9.5](#9-pertanyaan-terbuka--perlu-klarifikasi)).
4. Reseller melihat margin = (harga jual ke client) − (harga beli reseller dari admin).

### 8.2 Contoh Kasus

Produk: **SMS LBA Telkomsel**.

| Level | Harga ditetapkan oleh | HPP | Price | Keterangan |
|---|---|---|---|---|
| Default (semua client) | Admin | 400 | 500 | Berlaku untuk seluruh client kecuali di-override |
| Reseller X | Admin (override khusus utk Reseller X) | 400 | 450 | Reseller X belanja di harga 450 |
| Client Y (di bawah Reseller X) | Reseller X (override khusus utk Client Y) | — | 480 | Client Y belanja di harga 480 |

Dashboard Reseller X menampilkan margin: **480 − 450 = 30** per unit dari Client Y.

### 8.3 Model Data (usulan)

| Field (tabel `price_override`, usulan) | Tipe | Keterangan |
|---|---|---|
| `id` | integer | Primary key |
| `variant_id` | integer | FK ke `ProductVariant.id` (atau ke tier-nya jika tiering) |
| `set_by_user_id` | integer | User yang membuat override ini (admin atau reseller) |
| `target_user_id` | integer | User yang menerima harga ini (reseller, atau client di bawah reseller) |
| `price` | numeric | Harga override |
| `created_at` | timestamp | |

Resolusi harga untuk user tertentu: cari `price_override` dengan `target_user_id = current_user.id`; kalau tidak ada, naik satu level lewat `User.ReferralID` (harga reseller-nya); kalau masih tidak ada, pakai `ProductVariant.price` default.

---

## 9. Pertanyaan Terbuka / Perlu Klarifikasi

1. **Kedalaman pohon Product** — apakah *semua* level di §3 (termasuk level "operator" seperti TELKOMSEL/IOH) memang harus jadi row `Product` dengan `parent_id`, atau ada level yang sebaiknya jadi entity terpisah (mis. `Operator`) supaya query/filter lebih mudah?
2. **Field Variant** — catatan asli tidak merinci field `ProductVariant` selain `hpp`/`price`. Apakah perlu kode/SKU, satuan (mis. "per session", "per spot"), atau field lain?
3. **Tipe DOOH** — §3.4 mengasumsikan `type` untuk Billboard = `billboard` dan Mobile Truck = `mobile-videotron` berdasarkan data sample, bukan dari catatan asli (yang menulis "video tron" untuk ketiganya). Perlu konfirmasi nilai yang benar.
4. **Format `audience_id`** — aturan generate `AUD` + `YYMMDD` + suffix acak perlu didefinisikan persis (panjang suffix, charset, cek keunikan).
5. **Relasi reseller → client** — apakah memang memakai `User.ReferralID` (sudah ada di modul auth, lihat `internal/models/auth.go`) sebagai sumber hierarki "client di bawah reseller ini", atau butuh relasi terpisah?
6. **Sender ID sebagai entity** — SMS Broadcast dan WABA sama-sama punya langkah "pilih Sender ID yang sudah teregistrasi". Apakah `SenderID` perlu jadi entity/tabel sendiri (hasil dari alur registrasi §6), yang direferensikan oleh Audience/Campaign?
7. **Override tiering** — untuk variant dengan mode *tiering* ([§2.3](#23-product-variant)), apakah `price_override` di §8.3 override satu tier spesifik, atau seluruh set tier sekaligus?
