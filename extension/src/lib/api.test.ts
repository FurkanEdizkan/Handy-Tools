import { describe, it, expect } from 'vitest';
import { joinUrl, toSnake, fromSnake } from './api';

describe('joinUrl', () => {
  it('joins a base origin and an API path', () => {
    expect(joinUrl('http://127.0.0.1:8080', '/v1/health')).toBe('http://127.0.0.1:8080/v1/health');
  });

  it('tolerates a trailing slash on the base', () => {
    expect(joinUrl('http://127.0.0.1:8080/', '/v1/health')).toBe('http://127.0.0.1:8080/v1/health');
    expect(joinUrl('https://tools.example///', '/v1/jobs')).toBe('https://tools.example/v1/jobs');
  });
});

describe('toSnake / fromSnake', () => {
  it('round-trips nested objects through the wire format', () => {
    const camel = {
      uploadId: 'up_1',
      files: [{ name: 'a.png', path: '/in/a.png' }],
      outputDir: '/out',
      nested: { targetFormat: 'JPEG' },
    };
    const wire = toSnake(camel);
    expect(wire).toEqual({
      upload_id: 'up_1',
      files: [{ name: 'a.png', path: '/in/a.png' }],
      output_dir: '/out',
      nested: { target_format: 'JPEG' },
    });
    expect(fromSnake(wire)).toEqual(camel);
  });

  it('leaves primitives and nulls alone', () => {
    expect(toSnake(42)).toBe(42);
    expect(toSnake(null)).toBeNull();
    expect(fromSnake('hello')).toBe('hello');
  });
});
