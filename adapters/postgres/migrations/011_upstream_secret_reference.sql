-- Fresh deployments use managed environment references, never exposed secret bytes.
ALTER TABLE upstreams DROP COLUMN auth_value_encrypted;
ALTER TABLE upstreams ADD COLUMN auth_reference TEXT NOT NULL DEFAULT '';
