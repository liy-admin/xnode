<?php

namespace Plugin\MachineAutoNode;

use App\Services\Plugin\AbstractPlugin;

/**
 * Machine Auto Node.
 *
 * Adds POST /api/v2/server/machine/auto-node (see routes/api.php). The plugin
 * registers no hooks; all logic lives in Controllers\AutoNodeController.
 */
class Plugin extends AbstractPlugin
{
    public function boot(): void
    {
    }
}
