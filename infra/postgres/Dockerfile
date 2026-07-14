FROM postgres:18-bookworm

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        postgresql-${PG_MAJOR}-postgis-3 \
        postgresql-${PG_MAJOR}-postgis-3-scripts \
        postgis \
    && rm -rf /var/lib/apt/lists/*

LABEL org.opencontainers.image.title="Skyrix Delivery PostgreSQL"
LABEL org.opencontainers.image.description="PostgreSQL 18 with PostGIS for Skyrix Delivery"