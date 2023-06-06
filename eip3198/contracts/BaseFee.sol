// SPDX-License-Identifier: SEE LICENSE IN LICENSE
pragma solidity 0.8.12 ;
// Example from: https://twitter.com/solidity_lang/status/1425528304816332804/photo/1
contract BaseFee {
    function basefee_global() external view returns (uint) {
        return block.basefee;
    }

    function basefee_inline_assembly() external view returns (uint ret) {
        assembly {
            ret := basefee()
        }
    }
}