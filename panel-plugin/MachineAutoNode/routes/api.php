<?php

use Illuminate\Support\Facades\Route;
use Plugin\MachineAutoNode\Controllers\AutoNodeController;

Route::post('api/v2/server/machine/auto-node', [AutoNodeController::class, 'ensure']);
