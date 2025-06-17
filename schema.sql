-- This file contains the SQL schema for the components table.

CREATE TABLE components (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    parent_id UUID,
    FOREIGN KEY (parent_id) REFERENCES components(id) ON DELETE SET NULL
);
