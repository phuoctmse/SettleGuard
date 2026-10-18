import { fetchJson, ApiError } from './http';

describe('fetchJson', () => {
  beforeEach(() => {
    global.fetch = jest.fn();
  });

  it('returns parsed JSON on 200', async () => {
    (global.fetch as jest.Mock).mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ id: '1' }),
    });

    await expect(fetchJson('http://x/accounts/1')).resolves.toEqual({ id: '1' });
  });

  it('throws ApiError with status on non-2xx', async () => {
    (global.fetch as jest.Mock).mockResolvedValue({
      ok: false,
      status: 404,
      json: async () => ({}),
    });

    await expect(fetchJson('http://x/accounts/missing')).rejects.toMatchObject(
      new ApiError(404, 'http://x/accounts/missing'),
    );
  });
});
