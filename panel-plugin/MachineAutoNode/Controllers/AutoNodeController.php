<?php

namespace Plugin\MachineAutoNode\Controllers;

use App\Http\Controllers\Controller;
use App\Models\Plugin as PluginModel;
use App\Models\Server;
use App\Models\ServerMachine;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Cache;
use Illuminate\Support\Facades\File;
use Illuminate\Support\Facades\Log;

/**
 * Creates the first node for a machine that has none.
 *
 * Authenticated with the machine's own id + token (same as /server/machine/nodes).
 * The node end cannot choose protocol, groups or port: everything comes from the
 * plugin settings, so a leaked machine token can at most create one node on its
 * own machine, and only while that machine has no nodes at all.
 */
class AutoNodeController extends Controller
{
    private const PLUGIN_CODE = 'machine_auto_node';

    public function ensure(Request $request): JsonResponse
    {
        $request->validate([
            'machine_id' => 'required|integer',
            'token' => 'required|string',
            'host' => 'nullable|string|max:255',
        ]);

        $machine = ServerMachine::where('id', $request->input('machine_id'))
            ->where('token', $request->input('token'))
            ->first();
        if (!$machine || !$machine->is_active) {
            return response()->json(['message' => 'Machine not found or disabled'], 403);
        }

        $config = $this->loadConfig();
        if ($config === null) {
            return response()->json(['message' => 'Plugin is not enabled'], 404);
        }

        // Any node bound to this machine, including disabled ones, means an admin
        // already manages it; never add a second node behind their back.
        $existing = $this->machineNodeIds($machine);
        if (!empty($existing)) {
            return $this->result(false, $existing, 'Machine already has nodes');
        }

        if (!filter_var($config['auto_create'] ?? false, FILTER_VALIDATE_BOOLEAN)) {
            return $this->result(false, [], 'Auto creation is disabled in plugin settings');
        }

        $groupIds = $this->parseGroupIds($config['group_ids'] ?? '');
        if (empty($groupIds)) {
            return response()->json(['message' => 'Plugin setting group_ids is empty; refusing to create a node without groups'], 422);
        }

        $host = $this->resolveHost($request);
        if ($host === null) {
            return response()->json(['message' => 'Invalid host'], 422);
        }

        $port = (int) ($config['port'] ?? 443);
        if ($port < 1 || $port > 65535) {
            return response()->json(['message' => 'Plugin setting port is out of range'], 422);
        }

        $lock = Cache::lock('machine_auto_node:' . $machine->id, 30);
        if (!$lock->get()) {
            return response()->json(['message' => 'Another auto-node request for this machine is in progress'], 409);
        }

        try {
            // Re-check under the lock so concurrent requests cannot both create.
            $existing = $this->machineNodeIds($machine);
            if (!empty($existing)) {
                return $this->result(false, $existing, 'Machine already has nodes');
            }

            $keys = $this->generateRealityKeys();
            $serverName = trim((string) ($config['reality_server_name'] ?? '')) ?: 'www.microsoft.com';

            $server = Server::create([
                // type must come first: the protocol_settings mutator reads it.
                'type' => Server::TYPE_VLESS,
                'name' => $this->buildName((string) ($config['name_template'] ?? '{machine}'), $machine, $host),
                'group_ids' => $groupIds,
                'route_ids' => [],
                'rate' => (string) ($config['rate'] ?? '1'),
                'host' => $host,
                'port' => (string) $port,
                'server_port' => $port,
                'show' => filter_var($config['show'] ?? true, FILTER_VALIDATE_BOOLEAN),
                'enabled' => true,
                'machine_id' => $machine->id,
                'protocol_settings' => [
                    'tls' => 2,
                    'network' => 'tcp',
                    'flow' => 'xtls-rprx-vision',
                    'reality_settings' => [
                        'server_name' => $serverName,
                        'server_port' => '443',
                        'public_key' => $keys['public_key'],
                        'private_key' => $keys['private_key'],
                        'short_id' => $keys['short_id'],
                        'allow_insecure' => false,
                    ],
                    'utls' => [
                        'enabled' => true,
                        'fingerprint' => 'chrome',
                    ],
                ],
            ]);

            Log::info('machine_auto_node: created node', [
                'machine_id' => $machine->id,
                'node_id' => $server->id,
                'host' => $host,
                'port' => $port,
            ]);

            return $this->result(true, [$server->id], 'Node created', 201);
        } finally {
            $lock->release();
        }
    }

    /**
     * Reads the plugin row directly instead of HasPluginConfig, whose 1h cache is
     * not invalidated when an admin saves the settings.
     */
    private function loadConfig(): ?array
    {
        $plugin = PluginModel::where('code', self::PLUGIN_CODE)->where('is_enabled', true)->first();
        if (!$plugin) {
            return null;
        }

        $defaults = [];
        $configFile = dirname(__DIR__) . '/config.json';
        if (File::exists($configFile)) {
            $fields = json_decode(File::get($configFile), true)['config'] ?? [];
            foreach ($fields as $key => $field) {
                $defaults[$key] = $field['default'] ?? null;
            }
        }

        $stored = is_string($plugin->config) ? (json_decode($plugin->config, true) ?? []) : (array) $plugin->config;

        return array_merge($defaults, $stored);
    }

    private function machineNodeIds(ServerMachine $machine): array
    {
        return Server::where('machine_id', $machine->id)->orderBy('id')->pluck('id')->all();
    }

    private function parseGroupIds(mixed $value): array
    {
        if (is_array($value)) {
            $items = $value;
        } else {
            $items = preg_split('/[\s,，]+/u', (string) $value, -1, PREG_SPLIT_NO_EMPTY) ?: [];
        }

        $ids = [];
        foreach ($items as $item) {
            if (ctype_digit((string) $item) && (int) $item > 0) {
                // Stored as strings: ServerService matches users with whereJsonContains((string) group_id).
                $ids[] = (string) (int) $item;
            }
        }

        return array_values(array_unique($ids));
    }

    /**
     * Uses the host reported by the node when it is a valid IP or hostname,
     * otherwise the address the request came from.
     */
    private function resolveHost(Request $request): ?string
    {
        $reported = trim((string) $request->input('host', ''));
        if ($reported !== '') {
            if (filter_var($reported, FILTER_VALIDATE_IP)) {
                return $reported;
            }
            if (filter_var($reported, FILTER_VALIDATE_DOMAIN, FILTER_FLAG_HOSTNAME) && str_contains($reported, '.')) {
                return strtolower($reported);
            }
            return null;
        }

        $ip = $request->ip();
        return filter_var($ip, FILTER_VALIDATE_IP) ? $ip : null;
    }

    private function buildName(string $template, ServerMachine $machine, string $host): string
    {
        $name = strtr($template !== '' ? $template : '{machine}', [
            '{machine}' => (string) $machine->name,
            '{machine_id}' => (string) $machine->id,
            '{host}' => $host,
        ]);
        $name = trim($name);

        return $name !== '' ? mb_substr($name, 0, 255) : 'machine-' . $machine->id;
    }

    /**
     * Generates an X25519 key pair in the format xray / sing-box expect for
     * Reality: 32-byte keys, base64 URL-safe without padding.
     */
    private function generateRealityKeys(): array
    {
        $private = random_bytes(SODIUM_CRYPTO_SCALARMULT_SCALARBYTES);
        // Clamp like `xray x25519` so the stored private key is canonical.
        $private[0] = chr(ord($private[0]) & 248);
        $private[31] = chr((ord($private[31]) & 127) | 64);
        $public = sodium_crypto_scalarmult_base($private);

        return [
            'private_key' => self::base64Url($private),
            'public_key' => self::base64Url($public),
            'short_id' => bin2hex(random_bytes(8)),
        ];
    }

    private static function base64Url(string $bytes): string
    {
        return rtrim(strtr(base64_encode($bytes), '+/', '-_'), '=');
    }

    private function result(bool $created, array $nodeIds, string $message, int $status = 200): JsonResponse
    {
        return response()->json([
            'created' => $created,
            'node_ids' => array_values(array_map('intval', $nodeIds)),
            'message' => $message,
        ], $status);
    }
}
