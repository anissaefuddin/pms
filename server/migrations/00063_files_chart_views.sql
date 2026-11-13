-- +goose Up
ALTER TYPE view_type ADD VALUE IF NOT EXISTS 'files';
ALTER TYPE view_type ADD VALUE IF NOT EXISTS 'chart';
