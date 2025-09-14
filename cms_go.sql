--
-- PostgreSQL database dump
--

\restrict X6BbASzOoA3oeRkai4vRN4aH2Cu4lk01fALkZ7mpErD4iVME368z1DEJzDynOcM

-- Dumped from database version 17.6
-- Dumped by pg_dump version 17.6

-- Started on 2025-09-15 05:04:59

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- TOC entry 4931 (class 1262 OID 16552)
-- Name: cms_go; Type: DATABASE; Schema: -; Owner: postgres
--

CREATE DATABASE cms_go WITH TEMPLATE = template0 ENCODING = 'UTF8' LOCALE_PROVIDER = libc LOCALE = 'English_Indonesia.1252';


ALTER DATABASE cms_go OWNER TO postgres;

\unrestrict X6BbASzOoA3oeRkai4vRN4aH2Cu4lk01fALkZ7mpErD4iVME368z1DEJzDynOcM
\connect cms_go
\restrict X6BbASzOoA3oeRkai4vRN4aH2Cu4lk01fALkZ7mpErD4iVME368z1DEJzDynOcM

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- TOC entry 4925 (class 0 OID 16584)
-- Dependencies: 224
-- Data for Name: components; Type: TABLE DATA; Schema: public; Owner: postgres
--

INSERT INTO public.components VALUES (4, 'hero', '{
  "headline": "string",
   "subheadline": "string",
    "background": "string",
    "button_label":"string",
    "button_url":"string"
}', '<section class="hero">
    <div class="container hero-content">
      <h1 class="display-3 fw-bold">{{.headline}}</h1>
      <p class="lead mb-4">{{.subheadline}}</p>
      <a href="/signup" class="btn btn-primary btn-lg">Get Started</a>
    </div>
  </section>') ON CONFLICT DO NOTHING;
INSERT INTO public.components VALUES (2, 'content', '{
  "html": "string"
}
', '    <div class="container min-h-[80vh]">{{.html}}</div>
') ON CONFLICT DO NOTHING;
INSERT INTO public.components VALUES (3, 'footer', '{
  "text": "string"
}
', '<footer class="bg-gray-800 text-white border-t-[30px] border-x-[30px] border-red-600 rounded-t-[30px]">
  <!-- Content -->
  <div class="container mx-auto px-6 py-12 grid grid-cols-1 md:grid-cols-3 gap-8">
    
    <!-- Contact Us -->
    <div>
      <h3 class="font-bold mb-4">contact us</h3>
      <p class="mb-2">
        Masih bingung? atau ada kendala lain untuk menggunakan adsqoo,
        Anda bisa hubungi email dibawah ini.
      </p>
      <p class="mb-2 font-semibold">Email: admin@adsqoo.id</p>
      <button class="bg-red-600 text-white px-5 py-2 rounded-full mb-3 hover:bg-red-700">
        Contact Us
      </button>
      <p class="text-sm">
        Alamat: The Telkom Hub, Jl. Gatot Subroto, Kota Jakarta Selatan 12710
      </p>
    </div>

    <!-- Company -->
    <div>
      <h3 class="font-bold mb-4">company</h3>
      <ul class="space-y-2">
        <li><a href="#" class="hover:text-red-400">About Us</a></li>
        <li><a href="#" class="hover:text-red-400">Contact Us</a></li>
        <li><a href="#" class="hover:text-red-400">FAQ & Help</a></li>
        <li><a href="#" class="hover:text-red-400">Privacy & Policy</a></li>
        <li><a href="#" class="hover:text-red-400">Terms & Conditions</a></li>
      </ul>
      <div class="flex space-x-4 mt-4">
        <a href="#" aria-label="Facebook">🌐</a>
        <a href="#" aria-label="Instagram">📷</a>
        <a href="#" aria-label="LinkedIn">💼</a>
        <a href="#" aria-label="Twitter">🐦</a>
      </div>
    </div>

    <!-- Our Services -->
    <div>
      <h3 class="font-bold mb-4">our services</h3>
      <ul class="space-y-2">
        <li><a href="#" class="hover:text-red-400">SMS Advertising</a></li>
        <li><a href="#" class="hover:text-red-400">Outdoor Advertising</a></li>
        <li><a href="#" class="hover:text-red-400">Online Advertising</a></li>
        <li><a href="#" class="hover:text-red-400">Whatsapp Business</a></li>
        <li><a href="#" class="hover:text-red-400">Whatsapp Blast Bertarget</a></li>
        <li><a href="#" class="hover:text-red-400">Push Notification</a></li>
      </ul>
    </div>
  </div>

  <!-- Bottom -->
  <div class="bg-gray-900 text-center py-4 text-sm rounded-b-md">
    Copyright © 2025 - Adsqoo.id - All Right Reserved.
  </div>
</footer>
') ON CONFLICT DO NOTHING;
INSERT INTO public.components VALUES (1, 'header', '{
  "title": "string",
  "logo": "string",
  "links": [
    {"label": "string", "url": "string"}
  ]
}
', ' <style>
     #social-header {
      transition: transform 0.3s ease, opacity 0.3s ease;
    }
    #header {
      transition: top 0.3s ease;
    }
 </style>
 <div id="social-header" class="fixed top-0 left-0 w-full bg-gradient-to-r from-pink-600 to-purple-600 text-white text-sm py-1 z-50">
    <div class="container mx-auto flex justify-end gap-4">
      <a href="#" class="hover:underline">📘 Facebook</a>
      <a href="#" class="hover:underline">📸 Instagram</a>
      <a href="#" class="hover:underline">💼 LinkedIn</a>
      <a href="#" class="hover:underline">🐦 Twitter</a>
    </div>
  </div>

  <!-- Header main (fixed below social bar) -->
  <header id="header" class="fixed top-[2.5rem] left-0 w-full bg-white shadow z-40">
    <div class="container mx-auto flex items-center justify-between py-3 px-4">
      <!-- Logo -->
      <div class="flex items-center">
        <img src="/assets/logo.png" alt="Logo" class="h-10"/>
      </div>

      <!-- Nav -->
      <nav class="flex items-center gap-6">
        <div class="relative group">
          <button class="hover:text-purple-600">Adsqoo</button>
        </div>
        <div class="relative group">
          <button class="hover:text-purple-600">Product ▾</button>
          <div class="absolute left-0 mt-2 hidden w-48 bg-white border rounded shadow-lg group-hover:block">
            <a href="#" class="block px-4 py-2 hover:bg-gray-100">Messaging Services ▸</a>
            <a href="#" class="block px-4 py-2 hover:bg-gray-100">Online Services ▸</a>
            <a href="#" class="block px-4 py-2 hover:bg-gray-100">Outdoor Advertising</a>
          </div>
        </div>
        <div class="relative group">
          <button class="hover:text-purple-600">Solution ▾</button>
          <div class="absolute left-0 mt-2 hidden w-56 bg-white border rounded shadow-lg group-hover:block">
            <div class="relative group">
              <a href="#" class="block px-4 py-2 hover:bg-gray-100">Messaging Services ▸</a>
              <div class="absolute left-full top-0 mt-0 hidden w-56 bg-white border rounded shadow-lg group-hover:block">
                <a href="#" class="block px-4 py-2 hover:bg-gray-100 text-red-500">SMS Advertising</a>
                <a href="#" class="block px-4 py-2 hover:bg-gray-100">Whatsapp Business Platform</a>
                <a href="#" class="block px-4 py-2 hover:bg-gray-100">Whatsapp Blast Tertarget</a>
              </div>
            </div>
            <a href="#" class="block px-4 py-2 hover:bg-gray-100">Online Services ▸</a>
            <a href="#" class="block px-4 py-2 hover:bg-gray-100">Outdoor Advertising</a>
          </div>
        </div>
        <a href="#" class="hover:text-purple-600">Blog</a>
        <a href="#" class="hover:text-purple-600">Affiliate</a>
        <a href="#" class="hover:text-purple-600">Tutorial</a>
      </nav>

      <!-- Auth links -->
      <div class="flex items-center gap-4">
        <a href="#" class="hover:text-purple-600">Register</a>
        <a href="#" class="hover:text-purple-600 flex items-center gap-1">
          <span>⇨</span> Login
        </a>
      </div>
    </div>
  </header>
 <script>
    const socialHeader = document.getElementById(''social-header'');
    const header = document.getElementById(''header'');

    function toggleSocial() {
      if (window.scrollY === 0) {
        // at top → show social
        socialHeader.classList.remove(''-translate-y-full'', ''opacity-0'');
        header.style.top = socialHeader.offsetHeight + ''px'';
      } else {
        // scroll down → hide social
        socialHeader.classList.add(''-translate-y-full'', ''opacity-0'');
        header.style.top = ''0'';
      }
    }

    window.addEventListener(''scroll'', toggleSocial);
    window.addEventListener(''load'', toggleSocial);
  </script>') ON CONFLICT DO NOTHING;


--
-- TOC entry 4921 (class 0 OID 16564)
-- Dependencies: 220
-- Data for Name: layouts; Type: TABLE DATA; Schema: public; Owner: postgres
--

INSERT INTO public.layouts VALUES (1, 'front-layout', '{"name":"front-layout","rows":[{"columns":[{"components":[{"type":"header","props":{"title":"My CMS","logo":"/static/logo.png","links":[{"label":"Home","url":"/"},{"label":"Blog","url":"/blog"},{"label":"Contact","url":"/contact"}]}}]}]},{"columns":[{"components":[{"type":"content","props":{"html":"<p>This is CMS-powered content.</p>"}}]}]},{"columns":[{"components":[{"type":"footer","props":{"text":"© 2025 My CMS. All rights reserved."}}]}]}]}', '2025-09-13 20:08:43.836821+07', '<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>{{.Title}}</title>
  <meta name="viewport" content="width=device-width, initial-scale=1">
   <link rel="icon" type="image/x-icon" href="/assets/favicon.ico">
   <link href="https://fonts.googleapis.com/css2?family=Public+Sans:wght@400;500;600;700&display=swap" rel="stylesheet">
  
  <style>
    body { 
      font-family: ''Public Sans'', sans-serif; 
      margin: 0; 
      padding: 0; 
    }
    .row { display:flex; flex-wrap:wrap; margin-bottom:20px; }
    .col { flex:1; padding:10px; }
  </style>
  <script src="https://cdn.tailwindcss.com"></script>
</head>
<body>
{{ range .rows }}
  {{ range .columns }}
        {{ range .components }}
          {{ renderComponent .type .props }}
        {{ end }}
    {{ end }}
{{ end }}
</body>
</html>') ON CONFLICT DO NOTHING;


--
-- TOC entry 4923 (class 0 OID 16574)
-- Dependencies: 222
-- Data for Name: menus; Type: TABLE DATA; Schema: public; Owner: postgres
--



--
-- TOC entry 4919 (class 0 OID 16554)
-- Dependencies: 218
-- Data for Name: pages; Type: TABLE DATA; Schema: public; Owner: postgres
--

INSERT INTO public.pages VALUES (1, 'home', '/', 'page', '{
  "rows": [
    {
      "columns": [
        {
          "components": [
            {
              "type": "hero",
              "props": {
                "headline": "Welcome",
                "subheadline": "This hello",
                "background": "/static/bg.jpg",
                "button_label": "Start",
                "button_url": "/start"
              }
            }
          ]
        }
      ]
    },
    {
      "columns": [
        {
          "components": [
            {
              "type": "content",
              "props": {
                "html": "<p>Hello world content tesr</p>"
              }
            }
          ]
        }
      ]
    }
  ]
}
', 1, '2025-09-13 20:09:12.919776+07', '2025-09-14 18:52:58.273274+07') ON CONFLICT DO NOTHING;


--
-- TOC entry 4938 (class 0 OID 0)
-- Dependencies: 223
-- Name: components_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

SELECT pg_catalog.setval('public.components_id_seq', 4, true);


--
-- TOC entry 4939 (class 0 OID 0)
-- Dependencies: 219
-- Name: layouts_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

SELECT pg_catalog.setval('public.layouts_id_seq', 1, true);


--
-- TOC entry 4940 (class 0 OID 0)
-- Dependencies: 221
-- Name: menus_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

SELECT pg_catalog.setval('public.menus_id_seq', 1, false);


--
-- TOC entry 4941 (class 0 OID 0)
-- Dependencies: 217
-- Name: pages_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

SELECT pg_catalog.setval('public.pages_id_seq', 1, true);


-- Completed on 2025-09-15 05:04:59

--
-- PostgreSQL database dump complete
--

\unrestrict X6BbASzOoA3oeRkai4vRN4aH2Cu4lk01fALkZ7mpErD4iVME368z1DEJzDynOcM

