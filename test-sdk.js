import { Opencode } from '@opencode/sdk';

const client = new Opencode({
    baseURL: 'http://127.0.0.1:4096',
    fetch: async (url, options) => {
        console.log(`FETCH CALLED: ${options.method} ${url}`);
        return {
            ok: true,
            status: 200,
            json: async () => ({})
        };
    }
});

async function run() {
    await client.session.prompt({
        path: { id: "ses_123" },
        body: { parts: [{ type: "text", text: "hello" }] }
    });
}
run();
