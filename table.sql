CREATE TABLE IF NOT EXISTS mctrack_servers (
	name VARCHAR(128) NOT NULL,
	ip VARCHAR(261) NOT NULL, -- Domain max 255 + : + 5
	resolved_ip VARCHAR(51) NOT NULL, -- IPv6 max 45 + : + 5
	timestamp TIMESTAMPTZ NOT NULL,
	online INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS mctrack_watched_servers (
	id SERIAL NOT NULL,
	name VARCHAR(128) NOT NULL,
	ip VARCHAR(261) NOT NULL,
	PRIMARY KEY (id),
	UNIQUE (name)
);

SELECT create_hypertable('mctrack_servers', 'timestamp', if_not_exists => true);
