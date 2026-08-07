# Campaign system, Cart and Transaction
di sistemini aku ingin setiap request bisa menggunakan mode sandbox atau prod (api_key) from table users, perlu ditambahkan api_key_sandbox di table users
- userkey yang dipakai akan jadi definisi flag sandbox/prod mode untuk cart,order,transaction,audience,creative
- gunakan source web untuk saat ini nantinya akan ada (web,iframe,api)
- target utama  adalah halaman : campaign(order), cart,transaction,audience,creative bisa di embed di website reseller, atau api integrasi
- semua crud delete,hanya akan soft delete.

## Campaign page
1. buatkan beberapa halaman campaign (no template just tailwind html css) (will be serve as iframe with concept : {url}/campaign?key={userkey}) untuk role non admin
2. pindahkan campaign/add page dari dinamis page (db) existing ke harcoded version (router,handler,renderer html)
 - ubah pilihan campaign type dengan data dari product
   product category -> product (jika dipilih maka fetch api untuk ambil childrennya, jika ada tampilkan sebagain sub, ulangi sampai habis) masukkan price terakhir dari pilihan ke kalkulasi budget
 - ubah di tergetting (audience) menjadi tab (New, dan use existing)
 - ubah di media (creative) menjadi tab (New, dan use existing)
3. tapilan awal campaign adalah card list campaign, with stats, pagination, filter, dan add button
4. buatkan strukturnya menjadi ini :
di client:
- {url}/campaign?key={userkey}
- {url}/campaign/add?key={userkey}
- {url}/campaign/edit/260805Y4GK6W?key={userkey}
- {url}/campaign/detail/260805Y4GK6W?key={userkey}
di admin:
- {url}/admin/campaign?key={userkey}
- {url}/admin/campaign/add?key={userkey}
- {url}/admin/campaign/edit/260805Y4GK6W?key={userkey}
- {url}/admin/campaign/detail/260805Y4GK6W?key={userkey}

## Cart / order
dari page campaign/add page berisi form ads creation
order bersifat draftable, artinya ketika dibuat ada fitur auto save, dan bisa di lanjutkan ketika belum finish
- akan disimpan di tabel orders
- buat endpoint khusus untuk ini dengan payload json

struktur order :
- order_id generated YYMMDD{randomchar} (example 260805Y4GK6W)
- Campaign Name
- Campaign Description
- Product (wa-lba,wa-targetted,sms-lba,etc)
- Schedule
- Taxable flag (only product campaignable is taxable)
- Status
- Qty (grandtotal of qty on details or total qty on topupsms)
- details[] : (order campaign can be multi targetting and multi creative)
    - Audience_id
    - Creative_id
    - Qty (example 20000 sms on location A, 3000 sms on location B)
    - add other field if needed
- Grand Total
- original_cost (admin only dihitung dari hpp)
- reseller_cost (admin and reseller only, muncul di reseller , dihitung dari hpp reseller)
- sandbox (0/1) di hit dengan key sandbox/production
- source (web/api/iframe)
- add other field if needed

sample
![image](/Users/apple/Projects/cms-go/Screenshot 2026-08-07 at 06.55.10.png)

## Audience
ini adalah module untuk manajemen audience, berisi fields untuk kebutuhan campaign , property bisa berbeda sesuai jenis campaign, akan disimpan di tabel audiences,, buat endpoint crud, secara umum didefinisikan sebagai :
- audience_id AUD-YYMMDD{randomchar} example : AUD-260805EEUY67
- audience name
- product_type/product_id
- user_id
- location address
- min age
- max age
- gender
- interests 
- latitude
- longitude
- radius
- prov id
- kab id
- kec id
- kel id
- whitelist_phones
- file_url (for audience target csv/excel phones / emails)
- sandbox (0/1) di hit dengan key sandbox/production
- source (web/api/iframe)
- add other field if needed

tidak semua field akan terisi, akan menyesuaikan kebutuhan tipe product
- buat endpoint khusus untuk ini dengan payload json

## Creative
ini adalah module untuk manajemen creative, berisi fields untuk kebutuhan campaign , property bisa berbeda sesuai jenis campaign, akan disimpan di tabel creatives, buat endpoint crud, secara umum didefinisikan sebagai :
- creative_id CRE-YYMMDD{randomchar} example : CRE-260805EEUHU7
- creative name
- product_type/product_id
- user_id
- title
- caption
- body
- footer_type (none,cta)
- cta_props (url,phone,copy,quick-reply) example :{"type":"url","title":"Buy now","url":"aaa.com"}
- media_id
- file_url (external url file) example : resellerdomain/images/HQ.jpg
- sandbox (0/1) di hit dengan key sandbox/production
- source (web/api/iframe)
- add other field if needed

tidak semua field akan terisi, akan menyesuaikan kebutuhan tipe product
- buat endpoint khusus untuk ini dengan payload json

## Transaction
buat menjadi endpoint dengan fitur crud
- Transaksi berupa post api dengan isi orderids []
- konsepnya di halaman cart user bisa pilih 1 atau lebih cart(order secara bersamaan)
- akan disimpan di tabel transactions dengan kode TRX-YYMMDD{randomchar} example : TRX-260805EEDDJ6 
- sandbox (0/1) di hit dengan key sandbox/production
- source (web/api/iframe)

- ketika sudah dibuat maka akan muncul halaman invoice
- buat halaman invoice di {url}/invoice/{kodeinvoice} example : {url}/invoice/TRX-260805EEDDJ6/
![image](/Users/apple/Projects/cms-go/Screenshot 2026-08-07 at 06.58.09.png)
- buat endpoint khusus untuk ini dengan payload json

## statuses
0 Draft
1 Menunggu Moderasi
2 Sedang dalam proses moderasi
3 Menunggu waktu tayang
4 Campaign ditolak
5 Sedang tayang
6 Sudah tayang
7 Lanjut ke proses pembayaran
8 Lakukan pembayaran
default Draft