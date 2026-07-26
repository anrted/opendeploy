-- Migration: 002_modules.sql
-- Creates the modules table for tracking installable module state.

CREATE TABLE IF NOT EXISTS modules (
    id           TEXT NOT NULL PRIMARY KEY,
    name         TEXT NOT NULL,
    version      TEXT,
    state        TEXT NOT NULL DEFAULT 'available'
                     CHECK (state IN (
                         'available', 'installing', 'installed',
                         'enabled', 'disabled', 'removing', 'error'
                     )),
    config       TEXT,
    installed_at TEXT,
    updated_at   TEXT NOT NULL
);
