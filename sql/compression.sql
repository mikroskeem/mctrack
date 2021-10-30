ALTER TABLE mctrack_servers SET (
	timescaledb.compress,
	timescaledb.compress_segmentby = 'name'
);

SELECT add_compress_chunks_policy('mctrack_servers', INTERVAL '7 days');
