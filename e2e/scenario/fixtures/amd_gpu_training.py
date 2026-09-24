"""Small FP32 training/reference check on every GPU in a driver-only VHD.

The container supplies all ROCm userspace. No host ROCm libraries are mounted.
This is a functional smoke test, not a throughput or model-quality benchmark.
"""

import copy
import json

import torch


def train_and_compare(device):
    torch.manual_seed(20260919)
    x = torch.randn(256, 32)
    target = x @ torch.randn(32, 16) * 0.1
    reference = torch.nn.Linear(32, 16)
    actual = copy.deepcopy(reference).to(device)
    reference_optimizer = torch.optim.SGD(reference.parameters(), lr=0.2)
    actual_optimizer = torch.optim.SGD(actual.parameters(), lr=0.2)
    gpu_x, gpu_target = x.to(device), target.to(device)
    losses = []
    for step in range(40):
        reference_optimizer.zero_grad(set_to_none=True)
        actual_optimizer.zero_grad(set_to_none=True)
        expected_output = reference(x)
        actual_output = actual(gpu_x)
        expected_loss = torch.nn.functional.mse_loss(expected_output, target)
        actual_loss = torch.nn.functional.mse_loss(actual_output, gpu_target)
        torch.testing.assert_close(actual_output.cpu(), expected_output, rtol=5e-5, atol=5e-6)
        torch.testing.assert_close(actual_loss.cpu(), expected_loss, rtol=5e-5, atol=5e-6)
        assert torch.isfinite(actual_loss).item(), (device, step, "nonfinite loss")
        expected_loss.backward()
        actual_loss.backward()
        for actual_parameter, expected_parameter in zip(actual.parameters(), reference.parameters()):
            torch.testing.assert_close(actual_parameter.grad.cpu(), expected_parameter.grad,
                                       rtol=5e-5, atol=5e-6)
        reference_optimizer.step()
        actual_optimizer.step()
        for actual_parameter, expected_parameter in zip(actual.parameters(), reference.parameters()):
            torch.testing.assert_close(actual_parameter.cpu(), expected_parameter,
                                       rtol=5e-5, atol=5e-6)
        losses.append(actual_loss.item())
    assert losses[-1] < losses[0] * 0.8, (device, "training did not converge", losses)
    return {"gpu": device.index, "steps": len(losses), "initial_loss": losses[0], "final_loss": losses[-1]}


def main():
    assert torch.__version__ == "2.13.0+rocm10.0.0", torch.__version__
    assert torch.version.hip == "7.15.26333", torch.version.hip
    assert torch.cuda.is_available(), "ROCm GPU unavailable"
    assert torch.cuda.device_count() == 8, torch.cuda.device_count()
    torch.set_num_threads(2)
    torch.backends.cuda.matmul.allow_tf32 = False
    torch.backends.cudnn.allow_tf32 = False
    results = []
    for index in range(8):
        properties = torch.cuda.get_device_properties(index)
        assert properties.gcnArchName.split(":")[0] == "gfx942", properties
        assert "MI300X" in properties.name, properties
        for peer in range(8):
            if peer != index:
                assert torch.cuda.can_device_access_peer(index, peer), (index, peer)
        result = train_and_compare(torch.device("cuda", index))
        results.append(result)
        print(json.dumps(result), flush=True)
    print("AMDGPU_TRAINING_PASS " + json.dumps({
        "torch": torch.__version__, "hip": torch.version.hip,
        "gpu_count": len(results), "steps_per_gpu": 40,
        "checked": ["outputs", "loss", "gradients", "updated_parameters", "loss_decrease"],
        "rtol": 5e-5, "atol": 5e-6,
    }), flush=True)


if __name__ == "__main__":
    main()
