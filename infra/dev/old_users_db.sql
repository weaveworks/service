--
-- PostgreSQL database dump
--

SET statement_timeout = 0;
SET lock_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SET check_function_bodies = false;
SET client_min_messages = warning;

--
-- Name: plpgsql; Type: EXTENSION; Schema: -; Owner: 
--

CREATE EXTENSION IF NOT EXISTS plpgsql WITH SCHEMA pg_catalog;


--
-- Name: EXTENSION plpgsql; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION plpgsql IS 'PL/pgSQL procedural language';


SET search_path = public, pg_catalog;

--
-- Name: organizations_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE organizations_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE organizations_id_seq OWNER TO postgres;

SET default_tablespace = '';

SET default_with_oids = false;

--
-- Name: traceable; Type: TABLE; Schema: public; Owner: postgres; Tablespace: 
--

CREATE TABLE traceable (
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone
);


ALTER TABLE traceable OWNER TO postgres;

--
-- Name: organizations; Type: TABLE; Schema: public; Owner: postgres; Tablespace: 
--

CREATE TABLE organizations (
    id text DEFAULT nextval('organizations_id_seq'::regclass) NOT NULL,
    name text,
    probe_token text,
    first_probe_update_at timestamp with time zone
)
INHERITS (traceable);


ALTER TABLE organizations OWNER TO postgres;

--
-- Name: users_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE users_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE users_id_seq OWNER TO postgres;

--
-- Name: users; Type: TABLE; Schema: public; Owner: postgres; Tablespace: 
--

CREATE TABLE users (
    id text DEFAULT nextval('users_id_seq'::regclass) NOT NULL,
    email text,
    organization_id text,
    token text,
    token_created_at timestamp with time zone,
    approved_at timestamp with time zone,
    first_login_at timestamp with time zone
)
INHERITS (traceable);


ALTER TABLE users OWNER TO postgres;

--
-- Name: created_at; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY organizations ALTER COLUMN created_at SET DEFAULT now();


--
-- Name: updated_at; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY organizations ALTER COLUMN updated_at SET DEFAULT now();


--
-- Name: created_at; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY users ALTER COLUMN created_at SET DEFAULT now();


--
-- Name: updated_at; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY users ALTER COLUMN updated_at SET DEFAULT now();


--
-- Data for Name: organizations; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY organizations (created_at, updated_at, deleted_at, id, name, probe_token, first_probe_update_at) FROM stdin;
2015-10-14 08:40:37.529094+00	2015-10-14 08:40:37.523143+00	\N	2	thawing-leaf-77	r91ayzd8rr88ok1o7w5h7pcoz7hcesyf	\N
2015-10-14 08:40:51.374904+00	2015-10-14 08:40:51.36901+00	\N	3	small-feather-41	mxrqdoqhpm584skwgf8r9w17wrr35rq9	\N
2015-10-14 08:41:30.181113+00	2015-10-14 08:41:30.175131+00	\N	4	cold-pond-57	fkp8gf3wjo9eun3apez5a8z7a1eqa9g5	\N
2015-10-14 13:54:50.905685+00	2015-10-14 13:54:50.906982+00	\N	5	quiet-leaf-78	iaq6ijg6g9dzkhap1uqt5wbqgqtsd8x5	\N
2015-10-15 13:46:27.655097+00	2015-10-15 13:46:27.660313+00	\N	6	white-dust-47	gt1j8f6utze8i6r51su8s936q3wj1s5q	2015-10-15 14:31:41.775305+00
2015-10-15 17:17:41.688803+00	2015-10-15 17:17:41.69092+00	\N	7	polished-wildflower-97	5etdnkgye64q3qn6nq81wgz3k1zn7euy	2015-10-15 17:29:36.692178+00
2015-10-20 14:45:57.030967+00	2015-10-20 14:45:57.035772+00	\N	8	morning-nebula-66	3hud3h6ys3jhg9bq66n8xxa4b147dt5z	\N
2015-10-09 09:24:26.952562+00	2015-10-09 09:24:26.951206+00	\N	1	majestic-stone-74	daxur364yt5u9eib6h1ta61tmrsghstu	2015-11-26 10:44:51.827065+00
\.


--
-- Name: organizations_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

SELECT pg_catalog.setval('organizations_id_seq', 8, true);


--
-- Data for Name: traceable; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY traceable (created_at, updated_at, deleted_at) FROM stdin;
\.


--
-- Data for Name: users; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY users (created_at, updated_at, deleted_at, id, email, organization_id, token, token_created_at, approved_at, first_login_at) FROM stdin;
2015-10-08 17:23:36.344899+00	2015-10-08 17:23:36.342799+00	\N	4	david.kaltschmidt@gmail.com	\N	$2a$14$ibo82lIULMpniPwhSf/lF.Pai5vimcgOgaDo6tYHiJW3rBaztUKJW	2015-10-08 17:23:38.049289+00	\N	\N
2015-10-08 17:33:19.630052+00	2015-10-08 17:33:19.629999+00	\N	6	test01@pidster.com	\N	$2a$14$Th6W9FfIrtW3JffTQNfnEOQtvLYnhj13buRimhMzX40xh/yEQ7loi	2015-10-08 17:33:21.365464+00	\N	\N
2015-10-08 17:35:25.875273+00	2015-10-08 17:35:25.875592+00	\N	7	alfonsoacosta@gmail.com	\N	$2a$14$Z3qVTWAS1PUPBAigUQz0FuQJhHwdOy9i5jbzI8IX5mqtWirLIpf8C	2015-10-08 17:35:27.653411+00	\N	\N
2015-10-08 17:56:23.025567+00	2015-10-08 17:56:23.030453+00	\N	8	asdf@adfa.de	\N	$2a$14$okR6wtxT4p4tikJUrg8uKeCzZ7.eQqazWqsg9QzvrJP3gbMp4.09.	2015-10-08 17:56:24.80331+00	\N	\N
2015-10-14 08:40:03.166448+00	2015-10-14 08:40:03.16336+00	\N	9	peter@testing.domain.xyz	2	$2a$14$IkjFTv4eghYFxtCwAwIFRe2/RmjL4TflrlaHjCnFgOME3U.PuvLtG	2015-10-14 08:40:53.1613+00	2015-10-14 08:40:37.531857+00	\N
2015-10-14 08:41:00.13895+00	2015-10-14 08:41:00.135633+00	\N	10	peter+testing@weave.works	4		2015-10-14 08:42:04.280021+00	2015-10-14 08:41:30.183767+00	\N
2015-10-08 17:21:14.74767+00	2015-10-08 17:21:14.745051+00	\N	3	alfonso.acosta@gmail.com	5		2015-10-14 13:55:13.075092+00	2015-10-14 13:54:50.907793+00	\N
2015-10-08 17:19:58.061149+00	2015-10-08 17:19:58.05857+00	\N	2	tom@weave.works	6		2015-10-15 13:48:32.520859+00	2015-10-15 13:46:27.657858+00	\N
2015-10-08 17:25:09.596072+00	2015-10-08 17:25:09.594263+00	\N	5	peter@weave.works	7		2015-10-15 17:18:19.798026+00	2015-10-15 17:17:41.690505+00	\N
2015-10-16 10:36:46.362492+00	2015-10-16 10:36:46.362634+00	\N	11	tom@weave.work	\N	$2a$14$ynxDB5KqMw9zl6ks9JL5UeVXoGxh6oWjsJimwx86pLYK2ClFmNgKu	2015-10-16 10:36:48.471864+00	\N	\N
2015-10-20 14:45:24.826044+00	2015-10-20 14:45:24.832629+00	\N	12	fons+test@weave.works	8		2015-10-20 14:46:30.500907+00	2015-10-20 14:45:57.033273+00	\N
2015-10-23 15:54:30.732693+00	2015-10-23 15:54:30.735878+00	\N	13	tom.wilkie@gmail.com	\N	$2a$14$OTuonzKDZw22YFwdL6GpHum4Slm/qMkyUz8tjbIRS2HXdkNpjmyWe	2015-10-23 15:54:32.50697+00	\N	\N
2015-10-08 17:16:17.180724+00	2015-10-08 17:16:17.17891+00	\N	1	paul@weave.works	1	$2a$14$AZ2O.j8X3/7QK1NR3doa9.tgcsXDJADQzp2vPPumM1fH5Bg3jWCu6	2015-11-19 15:22:34.576045+00	2015-10-09 09:24:26.955773+00	\N
\.


--
-- Name: users_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

SELECT pg_catalog.setval('users_id_seq', 13, true);


--
-- Name: organizations_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres; Tablespace: 
--

ALTER TABLE ONLY organizations
    ADD CONSTRAINT organizations_pkey PRIMARY KEY (id);


--
-- Name: users_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres; Tablespace: 
--

ALTER TABLE ONLY users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: organizations_lower_name_idx; Type: INDEX; Schema: public; Owner: postgres; Tablespace: 
--

CREATE UNIQUE INDEX organizations_lower_name_idx ON organizations USING btree (lower(name)) WHERE (deleted_at IS NULL);


--
-- Name: organizations_probe_token_idx; Type: INDEX; Schema: public; Owner: postgres; Tablespace: 
--

CREATE UNIQUE INDEX organizations_probe_token_idx ON organizations USING btree (probe_token) WHERE (deleted_at IS NULL);


--
-- Name: users_lower_email_idx; Type: INDEX; Schema: public; Owner: postgres; Tablespace: 
--

CREATE UNIQUE INDEX users_lower_email_idx ON users USING btree (lower(email)) WHERE (deleted_at IS NULL);


--
-- Name: public; Type: ACL; Schema: -; Owner: postgres
--

REVOKE ALL ON SCHEMA public FROM PUBLIC;
REVOKE ALL ON SCHEMA public FROM postgres;
GRANT ALL ON SCHEMA public TO postgres;
GRANT ALL ON SCHEMA public TO PUBLIC;


--
-- PostgreSQL database dump complete
--

